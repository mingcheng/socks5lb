/*!*
 * Copyright (c) 2025 Hangzhou Guanwaii Technology Co., Ltd.
 *
 * This source code is licensed under the MIT License,
 * which is located in the LICENSE file in the source tree's root directory.
 *
 * File: pool.go
 * Author: mingcheng (mingcheng@apache.org)
 * File Created: Tuesday, June 21st 2022, 6:03:26 pm
 *
 * Modified By: mingcheng (mingcheng@apache.org)
 * Last Modified: 2025-10-07 11:22:29
 */

package socks5lb

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	log "github.com/sirupsen/logrus"
)

type Pool struct {
	current  atomic.Uint64
	backends map[string]*Backend
	lock     sync.RWMutex // Use RWMutex for better read concurrency
}

// Add add a backend to the pool
func (b *Pool) Add(backend *Backend) error {
	return b.addBackends([]*Backend{backend})
}

func (b *Pool) addBackends(backends []*Backend) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	seen := make(map[string]struct{}, len(backends))
	for _, backend := range backends {
		if backend == nil || strings.TrimSpace(backend.Addr) == "" {
			return fmt.Errorf("backend address is empty")
		}
		if b.backends[backend.Addr] != nil {
			return fmt.Errorf("backend %s already exists, remove it first", backend.Addr)
		}
		if _, ok := seen[backend.Addr]; ok {
			return fmt.Errorf("backend %s is duplicated in the batch", backend.Addr)
		}
		seen[backend.Addr] = struct{}{}
	}

	if b.backends == nil {
		b.backends = make(map[string]*Backend, len(backends))
	}
	for _, backend := range backends {
		b.backends[backend.Addr] = backend
	}
	return nil
}

// Remove remove a backend from the pool
func (b *Pool) Remove(addr string) (err error) {
	b.lock.Lock()
	defer b.lock.Unlock()

	if b.backends[addr] == nil {
		return fmt.Errorf("backend %s does not exist", addr)
	}
	delete(b.backends, addr)
	return
}

// All returns all backends in the pool
func (b *Pool) All() (backends []*Backend) {
	b.lock.RLock()

	// Preallocate slice with exact capacity to avoid resizing
	backends = make([]*Backend, 0, len(b.backends))
	for _, v := range b.backends {
		backends = append(backends, v)
	}
	b.lock.RUnlock()
	sort.Slice(backends, func(i, j int) bool {
		return backends[i].Addr < backends[j].Addr
	})
	return
}

// AllHealthy returns all healthy backends from the pool
func (b *Pool) AllHealthy() (backends []*Backend) {
	all := b.All()
	backends = all[:0]
	for _, v := range all {
		if v.Alive() {
			backends = append(backends, v)
		}
	}
	return
}

// NextIndex returns the next index for load balancer round-robin algorithm
// Uses atomic operations for thread-safe index management
func (b *Pool) NextIndex() int {
	b.lock.RLock()
	backendCount := len(b.backends)
	b.lock.RUnlock()

	if backendCount == 0 {
		return 0
	}

	return int(b.current.Add(1) % uint64(backendCount))
}

// Next returns the next available healthy backend using round-robin algorithm
// Returns nil if no healthy backend is available
func (b *Pool) Next() *Backend {
	// Get all healthy backends
	backends := b.AllHealthy()
	log.Tracef("found %d available backends", len(backends))

	// No backends available
	if len(backends) == 0 {
		return nil
	}

	next := b.current.Add(1) - 1
	return backends[next%uint64(len(backends))]
}

// Check performs health checks on all backends in the pool
func (b *Pool) Check() {
	b.check(context.Background())
}

func (b *Pool) check(ctx context.Context) {
	for _, backend := range b.All() {
		if ctx.Err() != nil {
			return
		}
		err := backend.check(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Errorf("health check failed for backend %s: %v", backend.Addr, err)
		} else {
			log.Debugf("health check successful for backend %s", backend.Addr)
		}
	}
}

// NewPool creates an independent backend pool.
func NewPool(backends ...[]Backend) *Pool {
	pool := &Pool{
		backends: make(map[string]*Backend),
	}

	for _, backend := range backends {
		for _, b := range backend {
			b.SetAlive(b.CheckConfig.InitialAlive)
			if err := pool.Add(&b); err != nil {
				log.Error(err)
			}
		}
	}

	return pool
}
