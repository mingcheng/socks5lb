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
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"

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

	alive     int32 // Use atomic int32 for thread-safe status updates (1=alive, 0=dead)
	checkHTTP func(context.Context, string) error
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
func (b *Backend) Check() error {
	return b.check(context.Background())
}

func (b *Backend) check(ctx context.Context) error {
	alive, err := b.probe(ctx)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	b.SetAlive(alive)
	return err
}

func (b *Backend) probe(ctx context.Context) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if url := b.CheckConfig.CheckURL; url != "" {
		checkHTTP := b.httpHealthCheck
		if b.checkHTTP != nil {
			checkHTTP = b.checkHTTP
		}
		err := checkHTTP(ctx, url)
		return err == nil, err
	}
	return b.CheckConfig.InitialAlive, nil
}

// httpHealthCheck performs HTTP-based health check through SOCKS5 proxy
func (b *Backend) httpHealthCheck(ctx context.Context, url string) error {
	client, err := b.httpProxyClient()
	if err != nil {
		return err
	}
	defer client.CloseIdleConnections()
	return checkHTTPHealth(ctx, client, url)
}

func checkHTTPHealth(ctx context.Context, client *http.Client, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Accept 2xx and 3xx status codes as healthy
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return nil
	}

	return fmt.Errorf("health check returned HTTP status %s", resp.Status)
}

// httpProxyClient creates an HTTP client configured to use the SOCKS5 proxy
func (b *Backend) httpProxyClient() (*http.Client, error) {
	timeout := b.CheckConfig.Timeout
	if timeout == 0 {
		timeout = DefaultCheckTimeout
	}
	duration, err := secondsDuration(uint64(timeout))
	if err != nil {
		return nil, fmt.Errorf("health check timeout: %w", err)
	}

	// Configure HTTP transport with SOCKS5 dialer
	httpTransport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return b.dialSocks5(ctx, network, addr, duration)
		},
		// Each health check creates a one-shot client, so idle connections cannot be reused.
		DisableKeepAlives:  true,
		DisableCompression: true,
	}

	return &http.Client{
		Transport: httpTransport,
		Timeout:   duration,
		// Don't follow redirects for health checks
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}, nil
}

// Socks5Conn creates a connection through the SOCKS5 proxy.
// The timeout, in seconds, covers dialing and negotiation, not the connection lifetime.
func (b *Backend) Socks5Conn(network, addr string, timeout int) (net.Conn, error) {
	if timeout < 0 {
		return nil, fmt.Errorf("SOCKS5 timeout must not be negative")
	}
	if timeout == 0 {
		timeout = DefaultCheckTimeout
	}
	duration, err := secondsDuration(uint64(timeout))
	if err != nil {
		return nil, fmt.Errorf("SOCKS5 timeout: %w", err)
	}
	return b.dialSocks5(context.Background(), network, addr, duration)
}

func (b *Backend) dialSocks5(ctx context.Context, network, addr string, timeout time.Duration) (conn net.Conn, err error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := socks5.NewClient(b.Addr, b.UserName, b.Password, 0, 0)
	if err != nil {
		return nil, err
	}

	var opened []net.Conn
	var stopCallbacks []func() bool
	defer func() {
		for _, stop := range stopCallbacks {
			stop()
		}
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		if err != nil {
			for _, socket := range opened {
				_ = socket.Close()
			}
			conn = nil
		}
	}()

	dial := func(network, _ string, address string) (net.Conn, error) {
		dialer := net.Dialer{KeepAlive: DefaultKeepAlivePeriod}
		socket, err := dialer.DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		opened = append(opened, socket)
		stopCallbacks = append(stopCallbacks, context.AfterFunc(ctx, func() {
			_ = socket.Close()
		}))
		return socket, nil
	}
	client.DialTCP = dial
	client.DialUDP = dial
	conn, err = client.Dial(network, addr)
	if err != nil {
		return nil, err
	}
	if network == "tcp" {
		// Return the underlying socket so transports can propagate TCP half-closes.
		return opened[0], nil
	}
	return conn, nil
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
