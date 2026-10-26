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
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	log "github.com/sirupsen/logrus"
)

// Pool selects healthy backends from an address-sorted membership snapshot.
// A Pool must not be copied after first use.
type Pool struct {
	current       atomic.Uint64
	backends      map[string]*Backend
	lock          sync.Mutex
	snapshot      atomic.Pointer[backendSnapshot]
	selectionPool sync.Pool
}

type backendSnapshot struct {
	backends []*Backend
}

// Add adds a backend to the pool. Its configuration must not be mutated afterward;
// use SetAlive to update health, or remove and re-add a replacement configuration.
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
	b.publishSnapshot()
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
	b.publishSnapshot()
	return
}

// publishSnapshot is called with b.lock held. Published slices are immutable.
func (b *Pool) publishSnapshot() {
	backends := make([]*Backend, 0, len(b.backends))
	for _, backend := range b.backends {
		backends = append(backends, backend)
	}
	slices.SortFunc(backends, func(a, b *Backend) int {
		return strings.Compare(a.Addr, b.Addr)
	})
	b.snapshot.Store(&backendSnapshot{backends: backends})
}

func (b *Pool) members() []*Backend {
	if snapshot := b.snapshot.Load(); snapshot != nil {
		return snapshot.backends
	}
	return nil
}

// All returns an address-sorted copy of the pool's membership.
func (b *Pool) All() []*Backend {
	return append([]*Backend{}, b.members()...)
}

// AllHealthy returns all healthy backends from the pool
func (b *Pool) AllHealthy() []*Backend {
	members := b.members()
	return appendHealthy(make([]*Backend, 0, len(members)), members, nil)
}

func appendHealthy(dst, members []*Backend, excluded map[string]struct{}) []*Backend {
	for _, backend := range members {
		if _, skip := excluded[backend.Addr]; !skip && backend.Alive() {
			dst = append(dst, backend)
		}
	}
	return dst
}

// NextIndex returns the next index for load balancer round-robin algorithm
// Uses atomic operations for thread-safe index management
func (b *Pool) NextIndex() int {
	backendCount := len(b.members())

	if backendCount == 0 {
		return 0
	}

	return int(b.current.Add(1) % uint64(backendCount))
}

// Next returns the next available healthy backend using round-robin algorithm
// Returns nil if no healthy backend is available
func (b *Pool) Next() *Backend {
	return b.next(nil)
}

func (b *Pool) next(excluded map[string]struct{}) *Backend {
	members := b.members()
	if len(members) == 0 {
		return nil
	}
	var selection *[]*Backend
	if cached := b.selectionPool.Get(); cached != nil {
		selection = cached.(*[]*Backend)
	} else {
		selection = new([]*Backend)
	}
	defer b.releaseSelection(selection)
	if cap(*selection) < len(members) {
		*selection = make([]*Backend, 0, len(members))
	}
	*selection = appendHealthy((*selection)[:0], members, excluded)
	backends := *selection
	if log.IsLevelEnabled(log.TraceLevel) {
		log.Tracef("found %d available backends", len(backends))
	}

	if len(backends) == 0 {
		return nil
	}

	next := b.current.Add(1) - 1
	return backends[next%uint64(len(backends))]
}

func (b *Pool) releaseSelection(selection *[]*Backend) {
	clear(*selection)
	*selection = (*selection)[:0]
	b.selectionPool.Put(selection)
}

// Check performs health checks on all backends in the pool
func (b *Pool) Check() {
	b.check(context.Background())
}

func (b *Pool) check(ctx context.Context) {
	runHealthChecks(ctx, b.members(), 1, func(ctx context.Context, backend *Backend) error {
		return backend.check(ctx)
	})
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
