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
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func NewProxyPool(t *testing.T) (pool *Pool, err error) {
	pool = NewPool()

	proxies := []string{
		"10.0.20.254:1086",
		"192.168.100.254:1086",
		"192.168.1.254:1086",
		"172.16.1.254:1086",
	}

	for _, v := range proxies {
		err = pool.Add(NewBackend(v, BackendCheckConfig{
			CheckURL:     "https://www.google.com/robots.txt",
			Timeout:      5,
			InitialAlive: false,
		}))
	}

	for i := 0; i < 100; i++ {
		p := NewPool()
		assert.Equal(t, &pool, &p, "proxyPool should be singleton")
	}

	return
}

func TestPool_HealthCheck(t *testing.T) {
	pool, _ := NewProxyPool(t)
	assert.NotNil(t, pool)

	pool.Check()
	for i := 0; i < 100; i++ {
		b := pool.Next()
		if b != nil {
			fmt.Printf("%v | ", pool.current)
			fmt.Printf("%v\n", b)
		}
	}
}

func TestPool_NextCheck(t *testing.T) {
	pool, _ := NewProxyPool(t)
	assert.NotNil(t, pool)

	for i := 0; i < 100; i++ {
		err := pool.Add(NewBackend(fmt.Sprintf("%d", i), BackendCheckConfig{
			InitialAlive: true,
		}))
		assert.NoError(t, err)
	}

	for i := 0; i < 100; i++ {
		next := pool.Next()
		assert.NotNil(t, next)
	}
}
