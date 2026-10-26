package socks5lb

import (
	"context"
	"math/rand/v2"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type healthStreak struct {
	successes int
	failures  int
}

type healthChecker struct {
	config HealthCheckConfig
	mu     sync.Mutex
	states map[*Backend]healthStreak
}

func newHealthChecker(config HealthCheckConfig) *healthChecker {
	config.Concurrency = max(1, config.Concurrency)
	config.SuccessThreshold = max(1, config.SuccessThreshold)
	config.FailureThreshold = max(1, config.FailureThreshold)
	return &healthChecker{config: config, states: make(map[*Backend]healthStreak)}
}

func (c *healthChecker) run(ctx context.Context, pool *Pool) {
	backends := pool.members()
	present := make(map[*Backend]struct{}, len(backends))
	for _, backend := range backends {
		present[backend] = struct{}{}
	}
	c.mu.Lock()
	for backend := range c.states {
		if _, exists := present[backend]; !exists {
			delete(c.states, backend)
		}
	}
	c.mu.Unlock()
	runHealthChecks(ctx, backends, c.config.Concurrency, c.check)
}

func (c *healthChecker) check(ctx context.Context, backend *Backend) error {
	alive, err := backend.probe(ctx)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if backend.CheckConfig.CheckURL == "" {
		backend.SetAlive(alive)
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	state := c.states[backend]
	if alive {
		state.failures = 0
		if state.successes < c.config.SuccessThreshold {
			state.successes++
		}
		if state.successes >= c.config.SuccessThreshold {
			backend.SetAlive(true)
		}
	} else {
		state.successes = 0
		if state.failures < c.config.FailureThreshold {
			state.failures++
		}
		if state.failures >= c.config.FailureThreshold {
			backend.SetAlive(false)
		}
	}
	c.states[backend] = state
	return err
}

func runHealthChecks(ctx context.Context, backends []*Backend, concurrency int, check func(context.Context, *Backend) error) {
	checkOne := func(backend *Backend) {
		if ctx.Err() != nil {
			return
		}
		err := check(ctx, backend)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			log.Errorf("health check failed for backend %s: %v", backend.Addr, err)
		} else {
			log.Debugf("health check successful for backend %s", backend.Addr)
		}
	}
	if concurrency <= 1 {
		for _, backend := range backends {
			if ctx.Err() != nil {
				return
			}
			checkOne(backend)
		}
		return
	}

	jobs := make(chan *Backend)
	var workers sync.WaitGroup
	for range min(concurrency, len(backends)) {
		workers.Go(func() {
			for backend := range jobs {
				checkOne(backend)
			}
		})
	}
	for _, backend := range backends {
		select {
		case <-ctx.Done():
			close(jobs)
			workers.Wait()
			return
		case jobs <- backend:
		}
	}
	close(jobs)
	workers.Wait()
}

func healthCheckJitter(interval time.Duration, percent int) time.Duration {
	maxDelay := interval / 100 * time.Duration(percent)
	if maxDelay == 0 {
		return 0
	}
	return time.Duration(rand.Int64N(int64(maxDelay) + 1))
}
