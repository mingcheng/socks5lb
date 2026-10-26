package main

import (
	"net"
	"testing"
	"time"

	"github.com/mingcheng/socks5lb"
	"github.com/stretchr/testify/require"
)

func TestProgramInitPreservesBackendConfiguration(t *testing.T) {
	p := &program{Config: &socks5lb.Configure{
		Backends: []socks5lb.Backend{{
			Addr:     "127.0.0.1:1080",
			UserName: "test-user",
			Password: "test-password",
			CheckConfig: socks5lb.BackendCheckConfig{
				InitialAlive: true,
			},
		}},
	}}
	require.NoError(t, p.Init(nil))
	backends := p.Server.Pool.All()
	require.Len(t, backends, 1)
	require.Equal(t, "test-user", backends[0].UserName)
	require.Equal(t, "test-password", backends[0].Password)
	require.True(t, backends[0].Alive())
}

func TestProgramInitRejectsDuplicateBackends(t *testing.T) {
	p := &program{Config: &socks5lb.Configure{
		Backends: []socks5lb.Backend{{Addr: "duplicate"}, {Addr: "duplicate"}},
	}}
	require.Error(t, p.Init(nil))
}

func TestProgramInitRequiresConfiguration(t *testing.T) {
	require.NotPanics(t, func() {
		require.Error(t, (&program{}).Init(nil))
	})
}

func TestProgramServerFailureStopsService(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer occupied.Close()
	p := &program{Config: &socks5lb.Configure{}}
	p.Config.ServerConfig.Sock5.Addr = occupied.Addr().String()
	require.NoError(t, p.Init(nil))
	require.NoError(t, p.Start())
	select {
	case <-p.Context().Done():
	case <-time.After(5 * time.Second):
		t.Fatal("server failure did not stop the service")
	}
	require.Error(t, p.Stop())
}

func TestProgramImmediateStop(t *testing.T) {
	for range 20 {
		p := &program{Config: &socks5lb.Configure{}}
		p.Config.ServerConfig.Sock5.Addr = "127.0.0.1:0"
		require.NoError(t, p.Init(nil))
		require.NoError(t, p.Start())
		require.NoError(t, p.Stop())
	}
}

func TestProgramLifecycleValidation(t *testing.T) {
	require.Error(t, (&program{}).Start())
	require.NoError(t, (&program{}).Stop())
	p := &program{Config: &socks5lb.Configure{}}
	p.Config.ServerConfig.Sock5.Addr = "127.0.0.1:0"
	require.NoError(t, p.Init(nil))
	require.NoError(t, p.Start())
	require.Error(t, p.Start())
	require.NoError(t, p.Stop())
}
