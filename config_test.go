package socks5lb

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServerConfigValidation(t *testing.T) {
	for _, tt := range []struct {
		name   string
		config ServerConfig
	}{
		{"connection limit", ServerConfig{Connections: ConnectionConfig{MaxConnections: -1}}},
		{"dial limit", ServerConfig{Connections: ConnectionConfig{MaxConcurrentDials: -1}}},
		{"attempts", ServerConfig{Connections: ConnectionConfig{DialAttempts: -1}}},
		{"concurrency", ServerConfig{HealthCheck: HealthCheckConfig{Concurrency: -1}}},
		{"failure threshold", ServerConfig{HealthCheck: HealthCheckConfig{FailureThreshold: -1}}},
		{"success threshold", ServerConfig{HealthCheck: HealthCheckConfig{SuccessThreshold: -1}}},
		{"negative jitter", ServerConfig{HealthCheck: HealthCheckConfig{JitterPercent: -1}}},
		{"excessive jitter", ServerConfig{HealthCheck: HealthCheckConfig{JitterPercent: 101}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Error(t, tt.config.Validate())
			server, err := NewServer(NewPool(), tt.config)
			require.Error(t, err)
			require.Nil(t, server)
		})
	}
	if uint64(^uint(0)) > uint64((1<<63-1)/1_000_000_000) {
		config := ServerConfig{Connections: ConnectionConfig{DialTimeout: ^uint(0)}}
		require.Error(t, config.Validate())
	}
	require.NoError(t, (ServerConfig{}).Validate())
	require.NoError(t, (ServerConfig{
		Connections: ConnectionConfig{MaxConnections: 10, MaxConcurrentDials: 2, DialAttempts: 3, DialTimeout: 1},
		HealthCheck: HealthCheckConfig{Concurrency: 4, CheckOnStart: true, JitterPercent: 100, FailureThreshold: 2, SuccessThreshold: 2},
	}).Validate())
}

func TestServerConnectionDefaultsAndIsolation(t *testing.T) {
	server, err := NewServer(NewPool(), ServerConfig{})
	require.NoError(t, err)
	require.Nil(t, server.connectionSlots)
	require.Nil(t, server.dialSlots)
	require.Equal(t, 1, server.dialAttempts)
	require.Equal(t, DefaultDialTimeout, server.dialTimeout)

	config := ServerConfig{Connections: ConnectionConfig{MaxConnections: 1, MaxConcurrentDials: 1}}
	first, err := NewServer(NewPool(), config)
	require.NoError(t, err)
	second, err := NewServer(NewPool(), config)
	require.NoError(t, err)
	first.connectionSlots <- struct{}{}
	first.dialSlots <- struct{}{}
	require.Empty(t, second.connectionSlots)
	require.Empty(t, second.dialSlots)
	<-first.connectionSlots
	<-first.dialSlots
}
