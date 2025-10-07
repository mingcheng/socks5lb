/*!*
 * Copyright (c) 2025 Hangzhou Guanwaii Technology Co., Ltd.
 *
 * This source code is licensed under the MIT License,
 * which is located in the LICENSE file in the source tree's root directory.
 *
 * File: backend.go
 * Author: mingcheng (mingcheng@apache.org)
 * File Created: Tuesday, June 21st 2022, 6:03:26 pm
 *
 * Modified By: mingcheng (mingcheng@apache.org)
 * Last Modified: 2025-10-07 11:21:59
 */

package socks5lb

import (
	"context"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/txthinking/socks5"
)

const (
	// DefaultCheckTimeout is the default timeout for health checks
	DefaultCheckTimeout = 10
)

type BackendCheckConfig struct {
	CheckURL     string `yaml:"check_url" json:"check_url"`
	InitialAlive bool   `yaml:"initial_alive" json:"initial_alive"`
	Timeout      uint   `yaml:"timeout" json:"timeout"`
}

type Backend struct {
	Addr        string             `yaml:"addr" json:"addr" binding:"required"`
	UserName    string             `yaml:"username" json:"username"`
	Password    string             `yaml:"password" json:"password"`
	CheckConfig BackendCheckConfig `yaml:"check_config" json:"check_config"`

	alive int32 // Use atomic int32 for thread-safe status updates (1=alive, 0=dead)
}

// Alive returns the current health status of the backend
// Uses atomic operation for thread-safe read
func (b *Backend) Alive() bool {
	return atomic.LoadInt32(&b.alive) == 1
}

// SetAlive atomically sets the backend's health status
func (b *Backend) SetAlive(alive bool) {
	if alive {
		atomic.StoreInt32(&b.alive, 1)
	} else {
		atomic.StoreInt32(&b.alive, 0)
	}
}

// Check performs health check on the backend by testing connectivity
// Returns error if the backend is not reachable or unhealthy
func (b *Backend) Check() (err error) {
	// If check URL is configured, use HTTP health check
	if url := b.CheckConfig.CheckURL; url != "" {
		err = b.httpHealthCheck(url)
		if err != nil {
			log.Errorf("HTTP health check failed for %s: %v", b.Addr, err)
			b.SetAlive(false)
			return
		}
		b.SetAlive(true)
		return
	}

	// Fall back to initial configuration if no check URL
	b.SetAlive(b.CheckConfig.InitialAlive)
	return
}

// httpHealthCheck performs HTTP-based health check through SOCKS5 proxy
func (b *Backend) httpHealthCheck(url string) error {
	client, err := b.httpProxyClient()
	if err != nil {
		return err
	}

	// Perform HEAD request to minimize bandwidth
	resp, err := client.Head(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Accept 2xx and 3xx status codes as healthy
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return nil
	}

	return err
}

// httpProxyClient creates an HTTP client configured to use the SOCKS5 proxy
func (b *Backend) httpProxyClient() (*http.Client, error) {
	timeout := b.CheckConfig.Timeout
	if timeout == 0 {
		timeout = DefaultCheckTimeout
	}

	// Configure HTTP transport with SOCKS5 dialer
	httpTransport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return b.Socks5Conn(network, addr, int(timeout))
		},
		// Connection pool settings for better performance
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 2,
		IdleConnTimeout:     30 * time.Second,
		// Disable compression to reduce CPU overhead
		DisableCompression: true,
	}

	return &http.Client{
		Transport: httpTransport,
		Timeout:   time.Duration(timeout) * time.Second,
		// Don't follow redirects for health checks
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}, nil
}

// socks5Client creates a SOCKS5 client with the specified timeout
func (b *Backend) socks5Client(timeout int) (*socks5.Client, error) {
	if timeout == 0 {
		timeout = DefaultCheckTimeout
	}
	return socks5.NewClient(string(b.Addr), b.UserName, b.Password, timeout, timeout)
}

// Socks5Conn creates a connection through the SOCKS5 proxy
func (b *Backend) Socks5Conn(network, addr string, timeout int) (cc net.Conn, err error) {
	client, err := b.socks5Client(timeout)
	if err != nil {
		return nil, err
	}

	return client.Dial(network, addr)
}

// NewBackend creates a new Backend instance with the specified configuration
func NewBackend(addr string, config BackendCheckConfig) (backend *Backend) {
	backend = &Backend{
		Addr:        addr,
		CheckConfig: config,
	}

	// Set initial alive status atomically
	backend.SetAlive(config.InitialAlive)

	return
}
