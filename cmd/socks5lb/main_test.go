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
