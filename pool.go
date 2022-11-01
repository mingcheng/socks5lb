/**
 * File: pool.go
 * Author: Ming Cheng<mingcheng@outlook.com>
 *
 * Created Date: Tuesday, June 21st 2022, 6:03:26 pm
 * Last Modified: Friday, July 15th 2022, 5:35:23 pm
 *
 * http://www.opensource.org/licenses/MIT
 */

package socks5lb

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"sync"
	"sync/atomic"
)

type Pool struct {
	backends map[string]*Backend
	current  uint64
	lock     sync.Mutex
}

// Add a backend to the pool
func (b *Pool) Add(backend *Backend) (err error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	if b.backends[backend.Addr] != nil {
		return fmt.Errorf("%v is already exists, remove it first", backend.Addr)
	}

	b.backends[backend.Addr] = backend
	return
}

// Remove a backend from the pool
func (b *Pool) Remove(addr string) (err error) {
	b.lock.Lock()
	defer b.lock.Unlock()

	if b.backends[addr] == nil {
		return fmt.Errorf("server %s is not exists", addr)
	}
	delete(b.backends, addr)
	return
}

// All returns all backends
func (b *Pool) All() (backends []*Backend) {
	for _, v := range b.backends {
		backends = append(backends, v)
	}
	return
}

// AllHealthy returns all healthy backends
func (b *Pool) AllHealthy() (backends []*Backend) {
	for _, v := range b.backends {
		if v.Alive() {
			backends = append(backends, v)
		}
	}

	return
}

// NextIndex returns the next index for loadbalancer interface
func (b *Pool) NextIndex() int {
	return int(atomic.AddUint64(&b.current, uint64(1)) % uint64(len(b.backends)))
}

// Next returns the next index in the pool if there is one available
// Only supports round-robin operations by default
func (b *Pool) Next() *Backend {

	// return healthy backends first
	backends := b.AllHealthy()
	log.Tracef("found all %d available backends", len(backends))

	// not found any backends available
	if len(backends) <= 0 {
		return nil
	}

	// loop entire backends to find out an Alive backend
	next := b.NextIndex()
	// start from next and move a full cycle
	l := len(backends) + next

	for i := next; i < l; i++ {
		// take an index by modding
		idx := i % len(backends)

		// if we have an alive backend, use it and store if its not the original one
		if backends[idx].Alive() {
			if i != next {
				atomic.StoreUint64(&b.current, uint64(idx))
			}

			return backends[idx]
		}
	}

	return nil
}

// Check if we have an alive backend
func (b *Pool) Check() {
	for _, v := range b.backends {
		backend := v
		go func() {
			err := backend.PeriodCheck()
			if err != nil {
				log.Errorf("check backend %s is failed, error %v", backend.Addr, err)
			} else {
				log.Debugf("check backend %s is successful", backend.Addr)
			}
		}()
	}
}

func (b *Pool) Close() error {
	for _, v := range b.backends {
		_ = v.StopCheck()
	}

	return nil
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
