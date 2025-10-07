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
	"fmt"
	"sync"
	"sync/atomic"

	log "github.com/sirupsen/logrus"
)

type Pool struct {
	current  uint64
	backends map[string]*Backend
	lock     sync.RWMutex // Use RWMutex for better read concurrency
}

// Add add a backend to the pool
func (b *Pool) Add(backend *Backend) (err error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	if b.backends[backend.Addr] != nil {
		return fmt.Errorf("%v is already exists, remove it first", backend.Addr)
	}

	b.backends[backend.Addr] = backend
	return
}

// Remove remove a backend from the pool
func (b *Pool) Remove(addr string) (err error) {
	b.lock.Lock()
	defer b.lock.Unlock()

	if b.backends[addr] == nil {
		return fmt.Errorf("server %s is not exists", addr)
	}
	delete(b.backends, addr)
	return
}

// All returns all backends in the pool
func (b *Pool) All() (backends []*Backend) {
	b.lock.RLock()
	defer b.lock.RUnlock()

	// Preallocate slice with exact capacity to avoid resizing
	backends = make([]*Backend, 0, len(b.backends))
	for _, v := range b.backends {
		backends = append(backends, v)
	}
	return
}

// AllHealthy returns all healthy backends from the pool
func (b *Pool) AllHealthy() (backends []*Backend) {
	b.lock.RLock()
	defer b.lock.RUnlock()

	// Preallocate slice with estimated capacity
	backends = make([]*Backend, 0, len(b.backends))
	for _, v := range b.backends {
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

	return int(atomic.AddUint64(&b.current, uint64(1)) % uint64(backendCount))
}

// Next returns the next available healthy backend using round-robin algorithm
// Returns nil if no healthy backend is available
func (b *Pool) Next() *Backend {
	// Get all healthy backends
	backends := b.AllHealthy()
	log.Tracef("found %d available backends", len(backends))

	// No backends available
	if len(backends) <= 0 {
		return nil
	}

	// Get starting index for round-robin
	next := b.NextIndex()

	// Loop through all backends starting from next index
	l := len(backends) + next

	for i := next; i < l; i++ {
		// Get index using modulo to wrap around
		idx := i % len(backends)

		// Return the first alive backend found
		if backends[idx].Alive() {
			// Update current index only if we moved from original position
			if i != next {
				atomic.StoreUint64(&b.current, uint64(idx))
			}

			return backends[idx]
		}
	}

	return nil
}

// Check performs health checks on all backends in the pool
func (b *Pool) Check() {
	b.lock.RLock()
	// Create a snapshot of backends to avoid holding lock during checks
	backends := make([]*Backend, 0, len(b.backends))
	for _, backend := range b.backends {
		backends = append(backends, backend)
	}
	b.lock.RUnlock()

	// Perform health checks without holding the lock
	for _, backend := range backends {
		err := backend.Check()
		if err != nil {
			log.Errorf("health check failed for backend %s: %v", backend.Addr, err)
		} else {
			log.Debugf("health check successful for backend %s", backend.Addr)
		}
	}
}

var (
	instance *Pool
	once     sync.Once
)

// NewPool instance for a new Pools instance
func NewPool(backends ...[]Backend) *Pool {
	once.Do(func() {
		instance = &Pool{
			backends: make(map[string]*Backend),
		}
	})

	for _, backend := range backends {
		for _, b := range backend {
			if err := instance.Add(&b); err != nil {
				log.Error(err)
			}
		}
	}

	return instance
}
