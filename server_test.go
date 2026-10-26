package socks5lb

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const networkTestTimeout = 5 * time.Second

func TestTransportPreservesHalfClose(t *testing.T) {
	client, proxyClient := tcpConnPair(t)
	proxyBackend, backend := tcpConnPair(t)
	defer client.Close()
	defer proxyClient.Close()
	defer proxyBackend.Close()
	defer backend.Close()

	transportDone := make(chan error, 1)
	go func() {
		transportDone <- (&Server{}).Transport(proxyClient, proxyBackend)
	}()

	backendDone := make(chan error, 1)
	go func() {
		request, err := io.ReadAll(backend)
		if err == nil {
			_, err = backend.Write(append([]byte("response:"), request...))
		}
		if closeErr := backend.CloseWrite(); err == nil {
			err = closeErr
		}
		backendDone <- err
	}()

	_, err := client.Write([]byte("payload"))
	require.NoError(t, err)
	require.NoError(t, client.CloseWrite())

	response, err := io.ReadAll(client)
	require.NoError(t, err)
	require.Equal(t, "response:payload", string(response))
	require.NoError(t, receiveError(t, backendDone))
	require.NoError(t, receiveError(t, transportDone))
}

func TestServerStopBeforeStart(t *testing.T) {
	require.NoError(t, (&Server{}).Stop())
}

func tcpConnPair(t testing.TB) (*net.TCPConn, *net.TCPConn) {
	t.Helper()

	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	dialed := make(chan *net.TCPConn, 1)
	dialErr := make(chan error, 1)
	go func() {
		conn, err := net.DialTCP("tcp", nil, listener.Addr().(*net.TCPAddr))
		if err != nil {
			dialErr <- err
			return
		}
		dialed <- conn
	}()

	accepted, err := listener.AcceptTCP()
	require.NoError(t, err)
	select {
	case conn := <-dialed:
		require.NoError(t, conn.SetDeadline(time.Now().Add(networkTestTimeout)))
		require.NoError(t, accepted.SetDeadline(time.Now().Add(networkTestTimeout)))
		return conn, accepted
	case err := <-dialErr:
		require.NoError(t, err)
	case <-time.After(networkTestTimeout):
		t.Fatal("timed out dialing local TCP listener")
	}
	return nil, nil
}

func receiveError(t testing.TB, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(networkTestTimeout):
		t.Fatal("timed out waiting for network operation")
		return nil
	}
}

func TestServerLifecycle(t *testing.T) {
	upstream, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = upstream.Close() })
	backendConn := make(chan net.Conn, 1)
	acceptErr := make(chan error, 1)
	go func() {
		conn, err := upstream.Accept()
		if err != nil {
			acceptErr <- err
			return
		}
		backendConn <- conn
	}()

	pool := NewPool([]Backend{{
		Addr: upstream.Addr().String(), CheckConfig: BackendCheckConfig{InitialAlive: true},
	}})
	server, done, socksAddr, httpAddr := startTestServer(t, pool)
	require.Error(t, server.Start(), "a second Start must not affect the running server")

	httpClient := &http.Client{Timeout: networkTestTimeout}
	t.Cleanup(httpClient.CloseIdleConnections)
	response, err := httpClient.Get("http://" + httpAddr + "/version")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.NoError(t, response.Body.Close())

	client, err := net.DialTimeout("tcp", socksAddr, networkTestTimeout)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	require.NoError(t, client.SetDeadline(time.Now().Add(networkTestTimeout)))
	_, err = client.Write([]byte("payload"))
	require.NoError(t, err)

	var remote net.Conn
	select {
	case remote = <-backendConn:
	case err := <-acceptErr:
		t.Fatalf("accept backend connection: %v", err)
	case <-time.After(networkTestTimeout):
		t.Fatal("backend connection was not established")
	}
	t.Cleanup(func() { _ = remote.Close() })
	require.NoError(t, remote.SetDeadline(time.Now().Add(networkTestTimeout)))
	payload := make([]byte, len("payload"))
	_, err = io.ReadFull(remote, payload)
	require.NoError(t, err)
	require.Equal(t, "payload", string(payload))

	var stops sync.WaitGroup
	for range 8 {
		stops.Go(func() { assert.NoError(t, server.Stop()) })
	}
	stops.Wait()
	require.NoError(t, receiveError(t, done))
	for _, conn := range []net.Conn{client, remote} {
		_, err := conn.Read(make([]byte, 1))
		require.ErrorIs(t, err, io.EOF)
	}
	for _, addr := range []string{socksAddr, httpAddr} {
		conn, err := net.DialTimeout("tcp", addr, networkTestTimeout)
		if conn != nil {
			_ = conn.Close()
		}
		require.Error(t, err, "listener %s remained open after Stop", addr)
	}
	server.mu.Lock()
	healthDone := server.healthDone
	activeConnections := len(server.connections)
	server.mu.Unlock()
	require.Empty(t, activeConnections)
	select {
	case <-healthDone:
	default:
		t.Fatal("health checker did not exit during Stop")
	}
	require.ErrorIs(t, server.Start(), net.ErrClosed)
}

func TestServerBindFailureCleansUp(t *testing.T) {
	for _, component := range []string{"socks5", "http"} {
		t.Run(component, func(t *testing.T) {
			occupied, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			defer occupied.Close()
			var config ServerConfig
			config.Sock5.Addr = "127.0.0.1:0"
			config.HTTP.Addr = "127.0.0.1:0"
			if component == "socks5" {
				config.Sock5.Addr = occupied.Addr().String()
			} else {
				config.HTTP.Addr = occupied.Addr().String()
			}
			server, err := NewServer(NewPool(), config)
			require.NoError(t, err)
			t.Cleanup(func() { assert.NoError(t, server.Stop()) })
			require.Error(t, server.Start())
			require.Nil(t, server.healthDone)
			if server.socks5Listener != nil {
				conn, err := net.DialTimeout("tcp", server.socks5Listener.Addr().String(), networkTestTimeout)
				if conn != nil {
					_ = conn.Close()
				}
				require.Error(t, err)
			}
			select {
			case <-server.stopDone:
			default:
				t.Fatal("failed startup did not finish cleanup")
			}
		})
	}
}

func TestServerStopCancelsHealthCheck(t *testing.T) {
	entered := make(chan struct{})
	backend := NewBackend("backend", BackendCheckConfig{
		CheckURL: "http://health.invalid/", InitialAlive: true,
	})
	backend.checkHTTP = func(ctx context.Context, _ string) error {
		close(entered)
		<-ctx.Done()
		return ctx.Err()
	}
	pool := NewPool()
	require.NoError(t, pool.Add(backend))
	server, err := NewServer(pool, ServerConfig{})
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, server.Stop()) })
	server.startHealthChecks(time.Millisecond)
	select {
	case <-entered:
	case <-time.After(networkTestTimeout):
		t.Fatal("health check did not start")
	}
	stopped := make(chan error, 1)
	go func() { stopped <- server.Stop() }()
	require.NoError(t, receiveError(t, stopped))
	require.True(t, backend.Alive(), "shutdown cancellation must not change backend health")
}

func TestServerStopDuringStartup(t *testing.T) {
	for range 20 {
		var config ServerConfig
		config.Sock5.Addr = "127.0.0.1:0"
		config.HTTP.Addr = "127.0.0.1:0"
		server, err := NewServer(NewPool(), config)
		require.NoError(t, err)
		done := make(chan error, 1)
		go func() { done <- server.Start() }()
		require.NoError(t, server.Stop())
		err = receiveError(t, done)
		require.True(t, err == nil || errors.Is(err, net.ErrClosed), "unexpected startup error: %v", err)
	}
}

func TestServerInvalidInitialization(t *testing.T) {
	_, err := NewServer(nil, ServerConfig{})
	require.Error(t, err)
	require.Error(t, (&Server{}).Start())
	require.Error(t, (&Server{}).AddBackend())
}

func TestTransportWithoutHalfClose(t *testing.T) {
	client, proxyClient := net.Pipe()
	proxyBackend, backend := net.Pipe()
	defer client.Close()
	defer proxyClient.Close()
	defer proxyBackend.Close()
	defer backend.Close()
	done := make(chan error, 1)
	go func() { done <- (&Server{}).Transport(proxyClient, proxyBackend) }()
	require.NoError(t, client.Close())
	require.NoError(t, receiveError(t, done))
}

func TestTransportClosesStreamsOnError(t *testing.T) {
	client, proxyClient := net.Pipe()
	proxyBackend, backend := net.Pipe()
	defer client.Close()
	defer proxyClient.Close()
	defer proxyBackend.Close()
	defer backend.Close()
	wantErr := errors.New("read failed")
	done := make(chan error, 1)
	go func() {
		done <- (&Server{}).Transport(&failingReadConn{Conn: proxyClient, err: wantErr}, proxyBackend)
	}()
	require.ErrorIs(t, receiveError(t, done), wantErr)
}

func TestCopyStreamPreservesCopyError(t *testing.T) {
	wantErr := errors.New("copy failed")
	result := make(chan error, 1)
	copyStream(closedWriteStream{Writer: io.Discard}, iotest.ErrReader(wantErr), result)
	require.ErrorIs(t, normalizeTransportError(<-result), wantErr)
}

func TestCopyDataBufferUsage(t *testing.T) {
	payload := bytes.Repeat([]byte("payload"), BufferSize)
	for _, mode := range []string{"writer-to", "reader-from", "buffered"} {
		t.Run(mode, func(t *testing.T) {
			original := bufferPool
			allocations := 0
			bufferPool = &sync.Pool{New: func() any {
				allocations++
				return new([BufferSize]byte)
			}}
			t.Cleanup(func() { bufferPool = original })

			var output bytes.Buffer
			var reader io.Reader = bytes.NewReader(payload)
			var writer io.Writer = &output
			if mode != "writer-to" {
				reader = readerOnly{reader}
			}
			if mode != "reader-from" {
				writer = writerOnly{writer}
			}
			n, err := copyData(writer, reader)
			require.NoError(t, err)
			require.Equal(t, int64(len(payload)), n)
			require.Equal(t, payload, output.Bytes())
			if mode == "buffered" {
				require.Equal(t, 1, allocations)
			} else {
				require.Zero(t, allocations, "fast paths must not borrow a copy buffer")
			}
		})
	}
}

type readerOnly struct{ io.Reader }
type writerOnly struct{ io.Writer }

type failingReadConn struct {
	net.Conn
	err error
}

func (c *failingReadConn) Read([]byte) (int, error) { return 0, c.err }

type closedWriteStream struct{ io.Writer }

func (closedWriteStream) CloseWrite() error { return net.ErrClosed }

func startTestServer(t *testing.T, pool *Pool) (*Server, <-chan error, string, string) {
	t.Helper()
	var config ServerConfig
	config.Sock5.Addr = "127.0.0.1:0"
	config.HTTP.Addr = "127.0.0.1:0"
	server, err := NewServer(pool, config)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() {
		defer close(done)
		done <- server.Start()
	}()
	t.Cleanup(func() {
		assert.NoError(t, server.Stop())
		assert.NoError(t, receiveError(t, done))
	})
	require.Eventually(t, func() bool {
		server.mu.Lock()
		defer server.mu.Unlock()
		return server.socks5Listener != nil && server.httpListener != nil && server.healthDone != nil
	}, networkTestTimeout, time.Millisecond)
	server.mu.Lock()
	socksAddr, httpAddr := server.socks5Listener.Addr().String(), server.httpListener.Addr().String()
	server.mu.Unlock()
	return server, done, socksAddr, httpAddr
}

func TestServerStandaloneListenersStop(t *testing.T) {
	for _, component := range []string{"socks5", "http"} {
		t.Run(component, func(t *testing.T) {
			server, err := NewServer(NewPool(), ServerConfig{})
			require.NoError(t, err)
			done := make(chan error, 1)
			go func() {
				defer close(done)
				if component == "http" {
					done <- server.ListenHTTPAdmin("127.0.0.1:0")
				} else {
					done <- server.ListenSocks5("127.0.0.1:0")
				}
			}()
			t.Cleanup(func() {
				assert.NoError(t, server.Stop())
				assert.NoError(t, receiveError(t, done))
			})
			var addr string
			require.Eventually(t, func() bool {
				server.mu.Lock()
				defer server.mu.Unlock()
				listener := server.socks5Listener
				if component == "http" {
					listener = server.httpListener
				}
				if listener == nil {
					return false
				}
				addr = listener.Addr().String()
				return true
			}, networkTestTimeout, time.Millisecond)
			conn, err := net.DialTimeout("tcp", addr, networkTestTimeout)
			require.NoError(t, err)
			require.NoError(t, conn.Close())
			require.NoError(t, server.Stop())
			require.NoError(t, receiveError(t, done))
		})
	}
}

func TestServerStopReportsCloseErrors(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	wantErr := errors.New("close failed")
	server := &Server{socks5Listener: &closeErrorListener{Listener: listener, err: wantErr}}
	require.ErrorIs(t, server.Stop(), wantErr)
	require.ErrorIs(t, server.Stop(), wantErr)
}

type closeErrorListener struct {
	net.Listener
	err error
}

func (l *closeErrorListener) Close() error {
	return errors.Join(l.Listener.Close(), l.err)
}
