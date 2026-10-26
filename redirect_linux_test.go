//go:build linux

package socks5lb

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOriginalDestinationKeepsConnectionOpen(t *testing.T) {
	client, server := tcpConnPair(t)
	defer client.Close()
	defer server.Close()
	addr, conn, err := getOriginalDstAddr(server)
	if err == nil {
		require.NotNil(t, addr)
		require.Same(t, server, conn)
	}
	sent := make(chan error, 1)
	go func() {
		_, err := client.Write([]byte("payload"))
		sent <- err
	}()
	payload := make([]byte, len("payload"))
	_, err = io.ReadFull(server, payload)
	require.NoError(t, err, "looking up the original destination must not close the caller's socket")
	require.Equal(t, "payload", string(payload))
	require.NoError(t, receiveError(t, sent))
}

func TestOriginalDestinationClosedConnection(t *testing.T) {
	client, server := tcpConnPair(t)
	defer client.Close()
	require.NoError(t, server.Close())
	_, _, err := getOriginalDstAddr(server)
	require.Error(t, err)
}

func TestTProxyInvalidConfiguration(t *testing.T) {
	require.Error(t, (&Server{}).ListenTProxy("127.0.0.1:0"))
	server, err := NewServer(NewPool(), ServerConfig{})
	require.NoError(t, err)
	defer server.Stop()
	require.Error(t, server.ListenTProxy("invalid:address:port"))
}
