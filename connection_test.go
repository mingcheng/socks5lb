package socks5lb

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDialBackendAttempts(t *testing.T) {
	for _, tt := range []struct {
		name      string
		attempts  int
		allFail   bool
		wantCalls int
		wantError bool
	}{
		{"default", 0, false, 1, true},
		{"one", 1, false, 1, true},
		{"retry", 2, false, 2, false},
		{"distinct backends only", 10, true, 2, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server, err := NewServer(NewPool([]Backend{
				{Addr: "a", CheckConfig: BackendCheckConfig{InitialAlive: true}},
				{Addr: "b", CheckConfig: BackendCheckConfig{InitialAlive: true}},
			}), ServerConfig{Connections: ConnectionConfig{DialAttempts: tt.attempts}})
			require.NoError(t, err)
			var addresses []string
			wantErr := errors.New("dial failed")
			server.dialContext = func(_ context.Context, network, addr string) (net.Conn, error) {
				require.Equal(t, "tcp", network)
				addresses = append(addresses, addr)
				if addr == "a" || tt.allFail {
					return nil, wantErr
				}
				conn, peer := net.Pipe()
				t.Cleanup(func() {
					_ = conn.Close()
					_ = peer.Close()
				})
				return conn, nil
			}
			conn, err := server.dialBackend(context.Background())
			if tt.wantError {
				require.ErrorIs(t, err, wantErr)
				require.Nil(t, conn)
			} else {
				require.NoError(t, err)
				require.NotNil(t, conn)
			}
			require.Len(t, addresses, tt.wantCalls)
			require.Equal(t, "a", addresses[0])
			if len(addresses) > 1 {
				require.Equal(t, "b", addresses[1])
			}
		})
	}
	server, err := NewServer(NewPool(), ServerConfig{})
	require.NoError(t, err)
	conn, err := server.dialBackend(context.Background())
	require.ErrorContains(t, err, "no healthy backend")
	require.Nil(t, conn)
}

func TestDialBackendSharedBudget(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		server, err := NewServer(NewPool([]Backend{
			{Addr: "a", CheckConfig: BackendCheckConfig{InitialAlive: true}},
			{Addr: "b", CheckConfig: BackendCheckConfig{InitialAlive: true}},
			{Addr: "c", CheckConfig: BackendCheckConfig{InitialAlive: true}},
		}), ServerConfig{Connections: ConnectionConfig{DialAttempts: 3, DialTimeout: 1}})
		require.NoError(t, err)
		var deadlines []time.Time
		failed := errors.New("first attempt failed")
		server.dialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			deadline, ok := ctx.Deadline()
			require.True(t, ok)
			deadlines = append(deadlines, deadline)
			if len(deadlines) == 1 {
				require.NoError(t, waitContext(ctx, 600*time.Millisecond))
				return nil, failed
			}
			<-ctx.Done()
			return nil, ctx.Err()
		}
		start := time.Now()
		conn, err := server.dialBackend(context.Background())
		require.Nil(t, conn)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.ErrorIs(t, err, failed)
		require.Equal(t, time.Second, time.Since(start))
		require.Len(t, deadlines, 2)
		require.Equal(t, deadlines[0], deadlines[1])
	})
}

func TestDialBackendConcurrencyAndQueueCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		server, err := NewServer(NewPool([]Backend{
			{Addr: "backend", CheckConfig: BackendCheckConfig{InitialAlive: true}},
		}), ServerConfig{Connections: ConnectionConfig{MaxConcurrentDials: 2}})
		require.NoError(t, err)
		var active, calls atomic.Int32
		server.dialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			calls.Add(1)
			active.Add(1)
			defer active.Add(-1)
			<-ctx.Done()
			return nil, ctx.Err()
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 3)
		for range 3 {
			go func() {
				_, err := server.dialBackend(ctx)
				done <- err
			}()
		}
		synctest.Wait()
		require.EqualValues(t, 2, active.Load())
		require.Len(t, server.dialSlots, 2)
		cancel()
		for range 3 {
			require.ErrorIs(t, <-done, context.Canceled)
		}
		require.Zero(t, active.Load())
		require.EqualValues(t, 2, calls.Load(), "queued requests must not dial after cancellation")
		require.Empty(t, server.dialSlots)
	})
}

func TestDialBackendQueueUsesBudget(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		server, err := NewServer(NewPool(), ServerConfig{
			Connections: ConnectionConfig{MaxConcurrentDials: 1, DialTimeout: 1},
		})
		require.NoError(t, err)
		server.dialSlots <- struct{}{}
		defer func() { <-server.dialSlots }()
		start := time.Now()
		conn, err := server.dialBackend(context.Background())
		require.Nil(t, conn)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Equal(t, time.Second, time.Since(start))
		require.Len(t, server.dialSlots, 1, "a timed-out waiter must not release another caller's slot")
	})
}

func TestServerConnectionLimitAndRelease(t *testing.T) {
	config := ServerConfig{Connections: ConnectionConfig{MaxConnections: 1}}
	config.Sock5.Addr = "127.0.0.1:0"
	server, err := NewServer(NewPool([]Backend{
		{Addr: "backend", CheckConfig: BackendCheckConfig{InitialAlive: true}},
	}), config)
	require.NoError(t, err)
	peers := make(chan net.Conn, 2)
	server.dialContext = func(context.Context, string, string) (net.Conn, error) {
		conn, peer := net.Pipe()
		peers <- peer
		return conn, nil
	}
	_, addr, _ := startConfiguredTestServer(t, server)
	first, err := net.DialTimeout("tcp", addr, networkTestTimeout)
	require.NoError(t, err)
	defer first.Close()
	var firstPeer net.Conn
	select {
	case firstPeer = <-peers:
	case <-time.After(networkTestTimeout):
		t.Fatal("first connection did not reach the backend")
	}
	defer firstPeer.Close()

	rejected, err := net.DialTimeout("tcp", addr, networkTestTimeout)
	require.NoError(t, err)
	defer rejected.Close()
	require.NoError(t, rejected.SetDeadline(time.Now().Add(networkTestTimeout)))
	_, err = rejected.Read(make([]byte, 1))
	require.Error(t, err)
	var timeout net.Error
	require.False(t, errors.As(err, &timeout) && timeout.Timeout(), "an over-limit connection must be closed promptly")
	require.Empty(t, peers, "rejected connections must not dial a backend")
	require.NoError(t, first.Close())
	require.Eventually(t, func() bool { return len(server.connectionSlots) == 0 }, networkTestTimeout, time.Millisecond)

	next, err := net.DialTimeout("tcp", addr, networkTestTimeout)
	require.NoError(t, err)
	defer next.Close()
	select {
	case peer := <-peers:
		defer peer.Close()
	case <-time.After(networkTestTimeout):
		t.Fatal("released connection slot was not reused")
	}
	require.NoError(t, server.Stop())
	require.Empty(t, server.connectionSlots)
}

func TestServerStopCancelsQueuedDials(t *testing.T) {
	config := ServerConfig{Connections: ConnectionConfig{MaxConnections: 3, MaxConcurrentDials: 1}}
	config.Sock5.Addr = "127.0.0.1:0"
	server, err := NewServer(NewPool([]Backend{
		{Addr: "backend", CheckConfig: BackendCheckConfig{InitialAlive: true}},
	}), config)
	require.NoError(t, err)
	var calls atomic.Int32
	server.dialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		calls.Add(1)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	_, addr, _ := startConfiguredTestServer(t, server)
	for range 3 {
		conn, err := net.DialTimeout("tcp", addr, networkTestTimeout)
		require.NoError(t, err)
		defer conn.Close()
	}
	require.Eventually(t, func() bool {
		return len(server.connectionSlots) == 3 && calls.Load() == 1
	}, networkTestTimeout, time.Millisecond)
	require.NoError(t, server.Stop())
	require.EqualValues(t, 1, calls.Load())
	require.Empty(t, server.connectionSlots)
	require.Empty(t, server.dialSlots)
	server.mu.Lock()
	require.Empty(t, server.connections)
	server.mu.Unlock()
}

func TestDialTimeoutDoesNotLimitConnectionLifetime(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	server, err := NewServer(NewPool([]Backend{{
		Addr: listener.Addr().String(), CheckConfig: BackendCheckConfig{InitialAlive: true},
	}}), ServerConfig{Connections: ConnectionConfig{DialTimeout: 1}})
	require.NoError(t, err)
	conn, err := server.dialBackend(context.Background())
	require.NoError(t, err)
	defer conn.Close()
	peer, err := listener.Accept()
	require.NoError(t, err)
	defer peer.Close()
	require.NoError(t, conn.SetDeadline(time.Now().Add(networkTestTimeout)))
	require.NoError(t, peer.SetDeadline(time.Now().Add(networkTestTimeout)))
	time.Sleep(1100 * time.Millisecond)
	_, err = conn.Write([]byte("still connected"))
	require.NoError(t, err)
	payload := make([]byte, len("still connected"))
	_, err = io.ReadFull(peer, payload)
	require.NoError(t, err)
	require.Equal(t, "still connected", string(payload))
}

func TestServerRetriesBeforeForwarding(t *testing.T) {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	defer listener.Close()
	require.NoError(t, listener.SetDeadline(time.Now().Add(networkTestTimeout)))
	upstreamDone := make(chan error, 1)
	go func() {
		conn, err := listener.AcceptTCP()
		if err != nil {
			upstreamDone <- err
			return
		}
		defer conn.Close()
		if err := conn.SetDeadline(time.Now().Add(networkTestTimeout)); err != nil {
			upstreamDone <- err
			return
		}
		payload, err := io.ReadAll(conn)
		if err == nil {
			_, err = conn.Write(append([]byte("response:"), payload...))
		}
		upstreamDone <- errors.Join(err, conn.CloseWrite())
	}()
	config := ServerConfig{Connections: ConnectionConfig{DialAttempts: 2}}
	config.Sock5.Addr = "127.0.0.1:0"
	server, err := NewServer(NewPool([]Backend{
		{Addr: "!invalid", CheckConfig: BackendCheckConfig{InitialAlive: true}},
		{Addr: listener.Addr().String(), CheckConfig: BackendCheckConfig{InitialAlive: true}},
	}), config)
	require.NoError(t, err)
	_, addr, _ := startConfiguredTestServer(t, server)
	client, err := net.DialTimeout("tcp", addr, networkTestTimeout)
	require.NoError(t, err)
	defer client.Close()
	require.NoError(t, client.SetDeadline(time.Now().Add(networkTestTimeout)))
	_, err = client.Write([]byte("payload"))
	require.NoError(t, err)
	closeWriter, ok := client.(interface{ CloseWrite() error })
	require.True(t, ok)
	require.NoError(t, closeWriter.CloseWrite())
	response, err := io.ReadAll(client)
	require.NoError(t, err)
	require.Equal(t, "response:payload", string(response))
	require.NoError(t, receiveError(t, upstreamDone))
}

type acceptResult struct {
	conn net.Conn
	err  error
}

type sequenceListener struct {
	results []acceptResult
	times   []time.Time
	calls   atomic.Int32
	closed  atomic.Bool
}

func (l *sequenceListener) Accept() (net.Conn, error) {
	l.calls.Add(1)
	l.times = append(l.times, time.Now())
	if l.closed.Load() || len(l.results) == 0 {
		return nil, net.ErrClosed
	}
	result := l.results[0]
	l.results = l.results[1:]
	return result.conn, result.err
}

func (l *sequenceListener) Close() error {
	l.closed.Store(true)
	return nil
}

func (l *sequenceListener) Addr() net.Addr { return &net.TCPAddr{} }

func TestAcceptBackoffAndReset(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client, proxy := net.Pipe()
		defer client.Close()
		defer proxy.Close()
		listener := &sequenceListener{results: []acceptResult{
			{err: os.ErrDeadlineExceeded},
			{conn: proxy},
			{err: os.ErrDeadlineExceeded},
		}}
		server, err := NewServer(NewPool(), ServerConfig{})
		require.NoError(t, err)
		start := time.Now()
		var handled atomic.Int32
		require.NoError(t, server.serveConnections(listener, func(net.Conn) { handled.Add(1) }))
		require.NoError(t, server.Stop())
		require.EqualValues(t, 1, handled.Load())
		require.Equal(t, 10*time.Millisecond, time.Since(start), "successful accepts must reset exponential backoff")
		require.True(t, listener.closed.Load())
	})
	synctest.Test(t, func(t *testing.T) {
		listener := &sequenceListener{}
		for range 10 {
			listener.results = append(listener.results, acceptResult{err: os.ErrDeadlineExceeded})
		}
		server, err := NewServer(NewPool(), ServerConfig{})
		require.NoError(t, err)
		require.NoError(t, server.serveConnections(listener, func(net.Conn) {}))
		for i := 1; i < len(listener.times); i++ {
			require.LessOrEqual(t, listener.times[i].Sub(listener.times[i-1]), time.Second)
		}
		last := len(listener.times) - 1
		require.Equal(t, time.Second, listener.times[last].Sub(listener.times[last-1]))
	})
}

func TestAcceptCancellationAndPermanentErrors(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		listener := &sequenceListener{results: []acceptResult{{err: os.ErrDeadlineExceeded}}}
		server, err := NewServer(NewPool(), ServerConfig{})
		require.NoError(t, err)
		require.NoError(t, server.registerListener(listener, &server.socks5Listener))
		done := make(chan error, 1)
		go func() { done <- server.serveConnections(listener, func(net.Conn) {}) }()
		synctest.Wait()
		stoppedAt := time.Now()
		require.NoError(t, server.Stop())
		require.NoError(t, <-done)
		require.Equal(t, stoppedAt, time.Now())
		require.EqualValues(t, 1, listener.calls.Load())
	})
	wantErr := errors.New("permanent accept failure")
	listener := &sequenceListener{results: []acceptResult{{err: wantErr}}}
	server, err := NewServer(NewPool(), ServerConfig{})
	require.NoError(t, err)
	require.ErrorIs(t, server.serveConnections(listener, func(net.Conn) {}), wantErr)
	require.EqualValues(t, 1, listener.calls.Load())
}
