/*!*
 * Copyright (c) 2025 Hangzhou Guanwaii Technology Co., Ltd.
 *
 * This source code is licensed under the MIT License,
 * which is located in the LICENSE file in the source tree's root directory.
 *
 * File: backend_test.go
 * Author: mingcheng (mingcheng@apache.org)
 * File Created: 2025-10-07 11:08:41
 *
 * Modified By: mingcheng (mingcheng@apache.org)
 * Last Modified: 2025-10-07 11:23:12
 */

package socks5lb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/txthinking/socks5"
)

func TestBackendCheckHTTPStatus(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantAlive  bool
		wantError  bool
	}{
		{name: "healthy", statusCode: http.StatusNoContent, wantAlive: true},
		{name: "unhealthy", statusCode: http.StatusServiceUnavailable, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.statusCode)
			}))
			defer server.Close()

			backend := NewBackend("127.0.0.1:1080", BackendCheckConfig{
				CheckURL:     server.URL,
				InitialAlive: !test.wantAlive,
			})
			backend.checkHTTP = func(ctx context.Context, url string) error {
				return checkHTTPHealth(ctx, server.Client(), url)
			}

			err := backend.Check()
			if test.wantError {
				require.Error(t, err)
				require.ErrorContains(t, err, http.StatusText(test.statusCode))
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, test.wantAlive, backend.Alive())
		})
	}
}

func TestBackendHealthThroughSOCKS5(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusFound, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodHead, r.Method)
				w.Header().Set("Location", "http://redirect.invalid/")
				w.WriteHeader(status)
			}))
			defer target.Close()

			proxyAddr, done := serveTestSOCKS5(t, func(conn net.Conn) error {
				if _, err := socks5.NewNegotiationRequestFrom(conn); err != nil {
					return err
				}
				if _, err := socks5.NewNegotiationReply(socks5.MethodUsernamePassword).WriteTo(conn); err != nil {
					return err
				}
				auth, err := socks5.NewUserPassNegotiationRequestFrom(conn)
				if err != nil {
					return err
				}
				if string(auth.Uname) != "test-user" || string(auth.Passwd) != "test-password" {
					return errors.New("backend credentials were not forwarded")
				}
				if _, err := socks5.NewUserPassNegotiationReply(socks5.UserPassStatusSuccess).WriteTo(conn); err != nil {
					return err
				}
				request, err := socks5.NewRequestFrom(conn)
				if err != nil {
					return err
				}
				if request.Cmd != socks5.CmdConnect || request.Address() != target.Listener.Addr().String() {
					return fmt.Errorf("unexpected SOCKS5 request: %s", request.Address())
				}
				upstream, err := net.DialTimeout("tcp", request.Address(), networkTestTimeout)
				if err != nil {
					return err
				}
				defer upstream.Close()
				if _, err := socks5.NewReply(socks5.RepSuccess, socks5.ATYPIPv4, []byte{127, 0, 0, 1}, []byte{0, 0}).WriteTo(conn); err != nil {
					return err
				}
				return (&Server{}).Transport(conn, upstream)
			})
			backend := NewBackend(proxyAddr, BackendCheckConfig{CheckURL: target.URL, Timeout: 1})
			backend.UserName, backend.Password = "test-user", "test-password"
			err := backend.Check()
			if status < 400 {
				require.NoError(t, err)
				require.True(t, backend.Alive())
			} else {
				require.ErrorContains(t, err, http.StatusText(status))
				require.False(t, backend.Alive())
			}
			require.NoError(t, receiveError(t, done))
		})
	}
}

func TestBackendHealthCancellationClosesNegotiation(t *testing.T) {
	negotiating := make(chan struct{})
	proxyAddr, proxyDone := serveTestSOCKS5(t, func(conn net.Conn) error {
		if _, err := socks5.NewNegotiationRequestFrom(conn); err != nil {
			return err
		}
		close(negotiating)
		_, err := conn.Read(make([]byte, 1))
		if !errors.Is(err, io.EOF) {
			return fmt.Errorf("negotiating connection was not closed: %w", err)
		}
		return nil
	})
	backend := NewBackend(proxyAddr, BackendCheckConfig{
		CheckURL: "http://health.invalid/", InitialAlive: true,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- backend.check(ctx) }()
	select {
	case <-negotiating:
	case <-time.After(networkTestTimeout):
		t.Fatal("SOCKS5 negotiation did not start")
	}
	cancel()
	require.ErrorIs(t, receiveError(t, done), context.Canceled)
	require.NoError(t, receiveError(t, proxyDone))
	require.True(t, backend.Alive())
}

func TestBackendNegotiationFailureClosesConnection(t *testing.T) {
	proxyAddr, done := serveTestSOCKS5(t, func(conn net.Conn) error {
		if _, err := socks5.NewNegotiationRequestFrom(conn); err != nil {
			return err
		}
		if _, err := socks5.NewNegotiationReply(0xff).WriteTo(conn); err != nil {
			return err
		}
		_, err := conn.Read(make([]byte, 1))
		if !errors.Is(err, io.EOF) {
			return fmt.Errorf("failed negotiation leaked its connection: %w", err)
		}
		return nil
	})
	backend := NewBackend(proxyAddr, BackendCheckConfig{})
	conn, err := backend.Socks5Conn("tcp", "127.0.0.1:80", 1)
	require.Error(t, err)
	require.Nil(t, conn)
	require.NoError(t, receiveError(t, done))
}

func TestBackendSOCKS5ConnectionSupportsHalfClose(t *testing.T) {
	proxyAddr, done := serveTestSOCKS5(t, func(conn net.Conn) error {
		if _, err := socks5.NewNegotiationRequestFrom(conn); err != nil {
			return err
		}
		if _, err := socks5.NewNegotiationReply(socks5.MethodNone).WriteTo(conn); err != nil {
			return err
		}
		if _, err := socks5.NewRequestFrom(conn); err != nil {
			return err
		}
		if _, err := socks5.NewReply(socks5.RepSuccess, socks5.ATYPIPv4, []byte{127, 0, 0, 1}, []byte{0, 0}).WriteTo(conn); err != nil {
			return err
		}
		payload, err := io.ReadAll(conn)
		if err != nil {
			return err
		}
		_, err = conn.Write(append([]byte("response:"), payload...))
		return err
	})
	conn, err := NewBackend(proxyAddr, BackendCheckConfig{}).Socks5Conn("tcp", "127.0.0.1:80", 1)
	require.NoError(t, err)
	defer conn.Close()
	require.NoError(t, conn.SetDeadline(time.Now().Add(networkTestTimeout)))
	closeWriter, ok := conn.(interface{ CloseWrite() error })
	require.True(t, ok, "SOCKS5 TCP connections must preserve half-close support")
	// The one-second negotiation timeout must not become a connection lifetime limit.
	<-time.After(1100 * time.Millisecond)
	_, err = conn.Write([]byte("payload"))
	require.NoError(t, err)
	require.NoError(t, closeWriter.CloseWrite())
	response, err := io.ReadAll(conn)
	require.NoError(t, err)
	require.Equal(t, "response:payload", string(response))
	require.NoError(t, receiveError(t, done))
}

func TestBackendCheckConfiguration(t *testing.T) {
	for _, alive := range []bool{false, true} {
		backend := NewBackend("backend", BackendCheckConfig{InitialAlive: alive})
		backend.SetAlive(!alive)
		require.NoError(t, backend.Check())
		require.Equal(t, alive, backend.Alive())
	}
	backend := NewBackend("backend", BackendCheckConfig{})
	client, err := backend.httpProxyClient()
	require.NoError(t, err)
	require.Equal(t, DefaultCheckTimeout*time.Second, client.Timeout)
	require.ErrorIs(t, client.CheckRedirect(nil, nil), http.ErrUseLastResponse)
	conn, err := backend.Socks5Conn("tcp", "127.0.0.1:80", -1)
	require.Error(t, err)
	require.Nil(t, conn)

	if uint64(^uint(0)) > uint64((1<<63-1)/int64(time.Second)) {
		backend.CheckConfig.Timeout = ^uint(0)
		_, err := backend.httpProxyClient()
		require.Error(t, err)
	}
}

func serveTestSOCKS5(t *testing.T, handle func(net.Conn) error) (string, <-chan error) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	var mu sync.Mutex
	var active net.Conn
	done := make(chan error, 1)
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			done <- normalizeTransportError(err)
			return
		}
		mu.Lock()
		active = conn
		mu.Unlock()
		defer conn.Close()
		if err := conn.SetDeadline(time.Now().Add(networkTestTimeout)); err != nil {
			done <- err
			return
		}
		done <- handle(conn)
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		mu.Lock()
		if active != nil {
			_ = active.Close()
		}
		mu.Unlock()
		assert.NoError(t, receiveError(t, done))
	})
	return listener.Addr().String(), done
}
