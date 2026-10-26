package socks5lb

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHealthCheckerConcurrencyAndCancellation(t *testing.T) {
	for _, concurrency := range []int{0, 1, 2, 20} {
		for _, cancelCheck := range []bool{false, true} {
			t.Run(fmt.Sprintf("concurrency=%d/cancel=%t", concurrency, cancelCheck), func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					release := make(chan struct{})
					var active, calls atomic.Int32
					pool := NewPool()
					for i := range 5 {
						backend := NewBackend(fmt.Sprintf("backend-%d", i), BackendCheckConfig{
							CheckURL: "http://health.invalid/", InitialAlive: cancelCheck,
						})
						backend.checkHTTP = func(ctx context.Context, _ string) error {
							calls.Add(1)
							active.Add(1)
							defer active.Add(-1)
							select {
							case <-ctx.Done():
								return ctx.Err()
							case <-release:
								return nil
							}
						}
						require.NoError(t, pool.Add(backend))
					}
					checker := newHealthChecker(HealthCheckConfig{Concurrency: concurrency})
					done := make(chan struct{})
					go func() {
						defer close(done)
						checker.run(ctx, pool)
					}()
					synctest.Wait()
					wantActive := min(max(concurrency, 1), 5)
					require.EqualValues(t, wantActive, active.Load())
					require.NoError(t, pool.Add(NewBackend("new", BackendCheckConfig{})))
					if cancelCheck {
						cancel()
					} else {
						close(release)
					}
					<-done
					require.Zero(t, active.Load())
					if cancelCheck {
						require.EqualValues(t, wantActive, calls.Load())
					} else {
						require.EqualValues(t, 5, calls.Load())
					}
					for _, backend := range pool.All() {
						if backend.Addr != "new" {
							require.True(t, backend.Alive(), "successful or canceled checks must preserve the expected health")
						}
					}
				})
			})
		}
	}
}

func TestHealthCheckerThresholds(t *testing.T) {
	failed := errors.New("unreachable")
	var probeErr error
	backend := NewBackend("backend", BackendCheckConfig{
		CheckURL: "http://health.invalid/", InitialAlive: true,
	})
	backend.checkHTTP = func(context.Context, string) error { return probeErr }
	pool := NewPool()
	require.NoError(t, pool.Add(backend))
	checker := newHealthChecker(HealthCheckConfig{FailureThreshold: 2, SuccessThreshold: 2})
	for _, step := range []struct {
		err   error
		alive bool
	}{
		{failed, true},
		{failed, false},
		{nil, false},
		{failed, false},
		{nil, false},
		{nil, true},
	} {
		probeErr = step.err
		checker.run(context.Background(), pool)
		require.Equal(t, step.alive, backend.Alive())
	}
	probeErr = failed
	require.ErrorIs(t, backend.Check(), failed)
	require.False(t, backend.Alive(), "direct Backend.Check must retain its immediate health semantics")
}

func TestHealthCheckerInitialPolicyAndReplacement(t *testing.T) {
	checker := newHealthChecker(HealthCheckConfig{FailureThreshold: 2, SuccessThreshold: 2})
	pool := NewPool([]Backend{
		{Addr: "healthy", CheckConfig: BackendCheckConfig{InitialAlive: true}},
		{Addr: "unhealthy"},
	})
	pool.All()[0].SetAlive(false)
	pool.All()[1].SetAlive(true)
	checker.run(context.Background(), pool)
	require.Equal(t, "healthy", pool.Next().Addr)
	require.Len(t, pool.AllHealthy(), 1)

	newFailingBackend := func() *Backend {
		backend := NewBackend("replaced", BackendCheckConfig{
			CheckURL: "http://health.invalid/", InitialAlive: true,
		})
		backend.checkHTTP = func(context.Context, string) error { return errors.New("unreachable") }
		return backend
	}
	first := newFailingBackend()
	require.NoError(t, pool.Add(first))
	checker.run(context.Background(), pool)
	require.True(t, first.Alive())
	require.NoError(t, pool.Remove(first.Addr))
	second := newFailingBackend()
	require.NoError(t, pool.Add(second))
	checker.run(context.Background(), pool)
	require.True(t, second.Alive(), "a replacement must not inherit the removed backend's failure streak")
	require.NotContains(t, checker.states, first)
	require.Len(t, checker.states, 1)
	require.NoError(t, pool.Remove(second.Addr))
	checker.run(context.Background(), pool)
	require.Empty(t, checker.states)
}

func TestHealthCheckScheduling(t *testing.T) {
	for _, immediate := range []bool{false, true} {
		t.Run(fmt.Sprintf("check_on_start=%t", immediate), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var checks atomic.Int32
				backend := NewBackend("backend", BackendCheckConfig{CheckURL: "http://health.invalid/"})
				backend.checkHTTP = func(context.Context, string) error {
					checks.Add(1)
					return nil
				}
				pool := NewPool()
				require.NoError(t, pool.Add(backend))
				server, err := NewServer(pool, ServerConfig{HealthCheck: HealthCheckConfig{CheckOnStart: immediate}})
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, server.Stop()) })
				server.startHealthChecks(time.Minute)
				synctest.Wait()
				var initial int32
				if immediate {
					initial = 1
				}
				require.Equal(t, initial, checks.Load())
				time.Sleep(time.Minute)
				synctest.Wait()
				require.Equal(t, initial+1, checks.Load())
				require.NoError(t, server.Stop())
				time.Sleep(time.Minute)
				require.Equal(t, initial+1, checks.Load())
			})
		})
	}
}

func TestHealthCheckJitterAndStop(t *testing.T) {
	for _, interval := range []time.Duration{time.Nanosecond, time.Minute, 1<<63 - 1} {
		for _, percent := range []int{0, 1, 25, 100} {
			for range 100 {
				delay := healthCheckJitter(interval, percent)
				require.GreaterOrEqual(t, delay, time.Duration(0))
				require.LessOrEqual(t, delay, interval/100*time.Duration(percent))
			}
		}
	}
	synctest.Test(t, func(t *testing.T) {
		server, err := NewServer(NewPool(), ServerConfig{HealthCheck: HealthCheckConfig{JitterPercent: 100}})
		require.NoError(t, err)
		server.startHealthChecks(time.Minute)
		synctest.Wait()
		time.Sleep(time.Minute)
		synctest.Wait()
		stoppedAt := time.Now()
		require.NoError(t, server.Stop())
		require.Equal(t, stoppedAt, time.Now(), "shutdown must cancel the jitter wait")
	})
}
