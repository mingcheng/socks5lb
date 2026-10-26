package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewConfig(t *testing.T) {
	for _, tt := range []struct {
		name, content string
		wantError     bool
	}{
		{"valid", "server:\n  socks5:\n    addr: '127.0.0.1:1080'\n", false},
		{"empty", "", true},
		{"null", "null\n", true},
		{"missing listener", "{}", true},
		{"blank listener", "server:\n  socks5:\n    addr: ' '\n", true},
		{"invalid YAML", "[", true},
		{"negative connection limit", "server:\n  socks5:\n    addr: '127.0.0.1:1080'\n  connections:\n    max_connections: -1\n", true},
		{"negative timeout", "server:\n  socks5:\n    addr: '127.0.0.1:1080'\n  connections:\n    dial_timeout: -1\n", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yml")
			require.NoError(t, os.WriteFile(path, []byte(tt.content), 0o600))
			config, err := NewConfig(path)
			if tt.wantError {
				require.Error(t, err)
				require.Nil(t, config)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, config)
			require.Equal(t, "127.0.0.1:1080", config.ServerConfig.Sock5.Addr)
		})
	}
}

func TestNewConfigMissingFile(t *testing.T) {
	config, err := NewConfig(filepath.Join(t.TempDir(), "missing.yml"))
	require.ErrorIs(t, err, os.ErrNotExist)
	require.Nil(t, config)
}

func TestNewConfigConnectionOptions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	require.NoError(t, os.WriteFile(path, []byte(`
server:
  socks5:
    addr: "127.0.0.1:1080"
  connections:
    max_connections: 100
    max_concurrent_dials: 4
    dial_attempts: 2
    dial_timeout: 5
`), 0o600))
	config, err := NewConfig(path)
	require.NoError(t, err)
	connections := config.ServerConfig.Connections
	require.Equal(t, 100, connections.MaxConnections)
	require.Equal(t, 4, connections.MaxConcurrentDials)
	require.Equal(t, 2, connections.DialAttempts)
	require.EqualValues(t, 5, connections.DialTimeout)
}
