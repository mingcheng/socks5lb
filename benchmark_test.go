package socks5lb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func benchmarkPool(b *testing.B, size int) *Pool {
	b.Helper()
	pool := NewPool()
	for i := range size {
		require.NoError(b, pool.Add(NewBackend(fmt.Sprintf("backend-%04d", i), BackendCheckConfig{
			InitialAlive: true,
		})))
	}
	return pool
}

func BenchmarkPoolNext(b *testing.B) {
	for _, size := range []int{1, 16, 256} {
		b.Run(fmt.Sprintf("backends=%d", size), func(b *testing.B) {
			pool := benchmarkPool(b, size)
			b.ReportAllocs()
			for b.Loop() {
				if pool.Next() == nil {
					b.Fatal("no healthy backend")
				}
			}
		})
	}
}

func BenchmarkPoolNextParallel(b *testing.B) {
	pool := benchmarkPool(b, 256)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if pool.Next() == nil {
				b.Error("no healthy backend")
				return
			}
		}
	})
}

func BenchmarkBackendDial(b *testing.B) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(b, err)
	done := make(chan error, 1)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					err = nil
				}
				done <- err
				return
			}
			if err := conn.Close(); err != nil {
				done <- err
				return
			}
		}
	}()
	b.Cleanup(func() {
		require.NoError(b, listener.Close())
		require.NoError(b, receiveError(b, done))
	})
	server, err := NewServer(NewPool([]Backend{{
		Addr: listener.Addr().String(), CheckConfig: BackendCheckConfig{InitialAlive: true},
	}}), ServerConfig{})
	require.NoError(b, err)
	b.ReportAllocs()
	for b.Loop() {
		conn, err := server.dialBackend(context.Background())
		if err != nil {
			b.Fatal(err)
		}
		if err := conn.Close(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTransport(b *testing.B) {
	for _, size := range []int{64, 4096, 65536} {
		b.Run(fmt.Sprintf("bytes=%d", size), func(b *testing.B) {
			client, proxyClient := tcpConnPair(b)
			proxyBackend, backend := tcpConnPair(b)
			for _, conn := range []net.Conn{client, proxyClient, proxyBackend, backend} {
				require.NoError(b, conn.SetDeadline(time.Time{}))
			}
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
				buf := make([]byte, size)
				for {
					if _, err := io.ReadFull(backend, buf); err != nil {
						if err == io.EOF {
							err = backend.CloseWrite()
						}
						backendDone <- err
						return
					}
					if _, err := backend.Write(buf); err != nil {
						backendDone <- err
						return
					}
				}
			}()

			payload := bytes.Repeat([]byte("x"), size)
			response := make([]byte, size)
			b.SetBytes(int64(2 * size))
			b.ReportAllocs()
			for b.Loop() {
				if _, err := client.Write(payload); err != nil {
					b.Fatal(err)
				}
				if _, err := io.ReadFull(client, response); err != nil {
					b.Fatal(err)
				}
			}
			require.Equal(b, payload, response)
			require.NoError(b, client.CloseWrite())
			require.NoError(b, receiveError(b, backendDone))
			require.NoError(b, receiveError(b, transportDone))
		})
	}
}
