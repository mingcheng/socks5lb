/*!*
 * Copyright (c) 2025 Hangzhou Guanwaii Technology Co., Ltd.
 *
 * This source code is licensed under the MIT License,
 * which is located in the LICENSE file in the source tree's root directory.
 *
 * File: server.go
 * Author: mingcheng (mingcheng@apache.org)
 * File Created: Wednesday, July 6th 2022, 5:39:05 pm
 *
 * Modified By: mingcheng (mingcheng@apache.org)
 * Last Modified: 2025-10-07 11:22:20
 */

package socks5lb

import (
	"io"
	"net"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	// BufferSize defines the size of buffers used for copying data between connections
	BufferSize = 32 * 1024 // 32KB buffer for better performance
)

var (
	// bufferPool reuses buffers to reduce GC pressure
	bufferPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, BufferSize)
		},
	}
)

// https://kasvith.me/posts/lets-create-a-simple-lb-go/

// Status tracks statistics and health information for connections
type Status struct {
	OutBytes, InBytes uint
	LastOnline        time.Time
	LastFailed        time.Time
	FailedTimes       uint
}

// Server represents the main SOCKS5 load balancer server
type Server struct {
	Pool   *Pool
	Config *ServerConfig

	healthCheckTimer *time.Ticker

	socks5Listener net.Listener
	tproxyListener net.Listener
}

// AddBackend adds a new backend to the server's pool
// TODO: Implementation needed
func (s *Server) AddBackend() error {
	return nil
}

// Start initializes and starts all server components
// - Health check timer for periodic backend monitoring
// - HTTP admin interface (if configured)
// - SOCKS5 proxy listener
func (s *Server) Start() (err error) {
	duration := SecFromEnv("CHECK_TIME_INTERVAL", 60)

	// Start periodic health check timer
	s.healthCheckTimer = time.NewTicker(duration)
	go func() {
		log.Infof("starting automatic backend health checks every %v", duration)
		for ; true; <-s.healthCheckTimer.C {
			s.Pool.Check()
		}
	}()

	//if s.Config.TProxy.Addr != "" {
	//	log.Tracef("start tproxy address on %s", s.Config.TProxy.Addr)
	//	go func() {
	//		if err = s.ListenTProxy(s.Config.TProxy.Addr); err != nil {
	//			log.Error(err)
	//		}
	//	}()
	//}

	// Start HTTP admin interface in separate goroutine if configured
	if s.Config.HTTP.Addr != "" {
		log.Tracef("starting HTTP admin interface on %s", s.Config.HTTP.Addr)
		go func() {
			if err = s.ListenHTTPAdmin(s.Config.HTTP.Addr); err != nil {
				log.Error(err)
			}
		}()
	}

	// Start SOCKS5 proxy server (blocks until error or shutdown)
	log.Tracef("starting SOCKS5 proxy on %s", s.Config.Sock5.Addr)
	return s.ListenSocks5(s.Config.Sock5.Addr)
}

// Stop gracefully shuts down the server and all listeners
func (s *Server) Stop() (e error) {
	log.Debug("initiating server shutdown")
	s.healthCheckTimer.Stop()

	// Close listeners asynchronously to avoid blocking
	if s.socks5Listener != nil {
		go s.socks5Listener.Close()
	}

	if s.tproxyListener != nil {
		go s.tproxyListener.Close()
	}

	return
}

// Transport bidirectionally copies data between dst and src connections
// Uses buffer pooling to reduce memory allocations and GC pressure
func (s *Server) Transport(dst, src io.ReadWriter) (err error) {
	// @see https://github.com/ginuerzh/gost/blob/0247b941ac31344f0d7b3c547941a051188ba202/server.go#L105

	// Buffered channel to collect errors from both goroutines
	errs := make(chan error, 2)

	// Copy from src to dst in one goroutine
	go func() {
		buf := bufferPool.Get().([]byte)
		defer bufferPool.Put(buf)

		_, err := io.CopyBuffer(dst, src, buf)
		errs <- err
	}()

	// Copy from dst to src in another goroutine
	go func() {
		buf := bufferPool.Get().([]byte)
		defer bufferPool.Put(buf)

		_, err := io.CopyBuffer(src, dst, buf)
		errs <- err
	}()

	// Wait for the first error (or completion)
	err = <-errs

	// EOF is expected when connection closes normally
	if err != nil && err == io.EOF {
		err = nil
	}

	log.Tracef("transport stream is finished")
	return
}

// NewServer creates a new Server instance with the given pool and configuration
func NewServer(pool *Pool, config ServerConfig) (*Server, error) {
	return &Server{
		Pool:   pool,
		Config: &config,
	}, nil
}
