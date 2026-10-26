/*!*
 * Copyright (c) 2025 Hangzhou Guanwaii Technology Co., Ltd.
 *
 * This source code is licensed under the MIT License,
 * which is located in the LICENSE file in the source tree's root directory.
 *
 * File: pool_test.go
 * Author: mingcheng (mingcheng@apache.org)
 * File Created: 2025-10-07 11:08:41
 *
 * Modified By: mingcheng (mingcheng@apache.org)
 * Last Modified: 2025-10-07 11:23:25
 */

package socks5lb

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func NewProxyPool(t *testing.T) *Pool {
	t.Helper()
	pool := NewPool()

	proxies := []string{
		"10.0.20.254:1086",
		"192.168.100.254:1086",
		"192.168.1.254:1086",
		"172.16.1.254:1086",
	}

	for _, addr := range proxies {
		require.NoError(t, pool.Add(NewBackend(addr, BackendCheckConfig{InitialAlive: true})))
	}

	return pool
}

func TestPool_HealthCheck(t *testing.T) {
	pool := NewPool([]Backend{
		{Addr: "healthy", CheckConfig: BackendCheckConfig{InitialAlive: true}},
		{Addr: "unhealthy"},
	})

	pool.Check()
	require.Len(t, pool.AllHealthy(), 1)
	require.Equal(t, "healthy", pool.Next().Addr)
}

func TestPool_NextCheck(t *testing.T) {
	pool := NewProxyPool(t)
	require.NotNil(t, pool)

	for i := 0; i < 10; i++ {
		err := pool.Add(NewBackend(fmt.Sprintf("backend-%02d", i), BackendCheckConfig{
			InitialAlive: true,
		}))
		require.NoError(t, err)
	}

	seen := make(map[string]bool)
	for i := 0; i < len(pool.AllHealthy()); i++ {
		next := pool.Next()
		require.NotNil(t, next)
		require.False(t, seen[next.Addr], "backend selected twice before round-robin cycle completed")
		seen[next.Addr] = true
	}
}

func TestNewPoolReturnsIndependentPools(t *testing.T) {
	first := NewPool()
	second := NewPool()
	require.NotSame(t, first, second)
}

func TestNewPoolInitialHealth(t *testing.T) {
	pool := NewPool([]Backend{
		{Addr: "healthy", CheckConfig: BackendCheckConfig{InitialAlive: true}},
		{Addr: "unhealthy"},
	})
	require.Len(t, pool.AllHealthy(), 1)
	require.Equal(t, "healthy", pool.Next().Addr)
}

func TestPoolZeroValueAndInvalidBackends(t *testing.T) {
	var pool Pool
	for _, backend := range []*Backend{nil, {Addr: ""}, {Addr: " "}} {
		require.NotPanics(t, func() {
			require.Error(t, pool.Add(backend))
		})
	}
	require.NotPanics(t, func() {
		require.NoError(t, pool.Add(NewBackend("healthy", BackendCheckConfig{InitialAlive: true})))
	})
	require.Len(t, pool.All(), 1)
	require.Error(t, pool.Add(NewBackend("healthy", BackendCheckConfig{})))
	require.NoError(t, pool.Remove("healthy"))
	require.Error(t, pool.Remove("healthy"))
	require.Nil(t, pool.Next())
	require.Equal(t, 0, pool.NextIndex())
}

func TestPoolConcurrentUpdates(t *testing.T) {
	pool := NewPool()
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Go(func() {
			backend := NewBackend(fmt.Sprintf("backend-%02d", i), BackendCheckConfig{InitialAlive: true})
			if !assert.NoError(t, pool.Add(backend)) {
				return
			}
			for range 20 {
				backend.SetAlive(false)
				pool.Next()
				pool.All()
				backend.SetAlive(true)
			}
			assert.NoError(t, pool.Remove(backend.Addr))
		})
	}
	wg.Wait()
	require.Empty(t, pool.All())
}

func TestPoolHealthCheckDoesNotHoldLock(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	backend := NewBackend("checking", BackendCheckConfig{CheckURL: "http://health.invalid/"})
	backend.checkHTTP = func(context.Context, string) error {
		close(entered)
		<-release
		return errors.New("unhealthy")
	}
	pool := NewPool()
	require.NoError(t, pool.Add(backend))
	checked := make(chan error, 1)
	go func() {
		pool.Check()
		checked <- nil
	}()
	t.Cleanup(func() {
		close(release)
		require.NoError(t, receiveError(t, checked))
	})
	select {
	case <-entered:
	case <-time.After(networkTestTimeout):
		t.Fatal("health check did not start")
	}
	updated := make(chan error, 1)
	go func() {
		updated <- pool.Add(NewBackend("new", BackendCheckConfig{InitialAlive: true}))
	}()
	require.NoError(t, receiveError(t, updated))
	require.Equal(t, "new", pool.Next().Addr)
	require.Equal(t, 0, pool.NextIndex())
}
