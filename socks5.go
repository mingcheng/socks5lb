/*!*
 * Copyright (c) 2025 Hangzhou Guanwaii Technology Co., Ltd.
 *
 * This source code is licensed under the MIT License,
 * which is located in the LICENSE file in the source tree's root directory.
 *
 * File: socks5.go
 * Author: mingcheng (mingcheng@apache.org)
 * File Created: Wednesday, July 6th 2022, 11:46:39 am
 *
 * Modified By: mingcheng (mingcheng@apache.org)
 * Last Modified: 2025-10-07 11:21:47
 */

package socks5lb

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	// DefaultDialTimeout is the default timeout for dialing backend connections
	DefaultDialTimeout = 10 * time.Second
	// DefaultKeepAlivePeriod is the interval for TCP keepalive probes
	DefaultKeepAlivePeriod = 30 * time.Second
)

// ListenSocks5 listens on a specific address and handles SOCKS5 connections
func (s *Server) ListenSocks5(addr string) error {
	listener, err := s.listenSocks5(addr)
	if err != nil {
		return err
	}
	return s.serveConnections(listener, s.handleSocks5Connection)
}

func (s *Server) listenSocks5(addr string) (net.Listener, error) {
	if s.Pool == nil {
		return nil, errors.New("backend pool is required")
	}
	if err := s.configureConnections(); err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen SOCKS5: %w", err)
	}
	if err := s.registerListener(listener, &s.socks5Listener); err != nil {
		_ = listener.Close()
		return nil, err
	}
	return listener, nil
}

// handleSocks5Connection processes a single SOCKS5 client connection
func (s *Server) handleSocks5Connection(socks5Conn net.Conn) {
	defer socks5Conn.Close()

	// Enable TCP keepalive to detect dead connections
	if tcpConn, ok := socks5Conn.(*net.TCPConn); ok {
		if err := tcpConn.SetKeepAlive(true); err != nil {
			log.Warnf("failed to set keepalive: %v", err)
		}
		if err := tcpConn.SetKeepAlivePeriod(DefaultKeepAlivePeriod); err != nil {
			log.Warnf("failed to set keepalive period: %v", err)
		}
	}

	backendConn, err := s.dialBackend(s.connectionContext())
	if err != nil {
		log.Errorf("failed to connect to a backend: %v", err)
		return
	}
	if !s.trackConnection(backendConn) {
		_ = backendConn.Close()
		return
	}
	defer s.releaseConnection(backendConn)

	// Transport data bidirectionally between client and backend
	if err := s.Transport(socks5Conn, backendConn); err != nil {
		log.Debugf("transport error: %v", err)
	}
}

func (s *Server) configureConnections() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return net.ErrClosed
	}
	if s.configured {
		return nil
	}
	var config ServerConfig
	if s.Config != nil {
		config = *s.Config
	}
	if err := config.Validate(); err != nil {
		return err
	}
	options := config.Connections
	s.dialTimeout = DefaultDialTimeout
	if options.DialTimeout != 0 {
		duration, err := secondsDuration(uint64(options.DialTimeout))
		if err != nil {
			return fmt.Errorf("dial timeout: %w", err)
		}
		s.dialTimeout = duration
	}
	s.dialAttempts = max(1, options.DialAttempts)
	dialer := &net.Dialer{KeepAlive: DefaultKeepAlivePeriod}
	s.dialContext = dialer.DialContext
	if options.MaxConnections > 0 {
		s.connectionSlots = make(chan struct{}, options.MaxConnections)
	}
	if options.MaxConcurrentDials > 0 {
		s.dialSlots = make(chan struct{}, options.MaxConcurrentDials)
	}
	s.configured = true
	return nil
}

func (s *Server) dialBackend(parent context.Context) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(parent, s.dialTimeout)
	defer cancel()
	if s.dialSlots != nil {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("wait for backend dial slot: %w", ctx.Err())
		case s.dialSlots <- struct{}{}:
		}
		defer func() { <-s.dialSlots }()
	}

	var attempted map[string]struct{}
	if s.dialAttempts > 1 {
		attempted = make(map[string]struct{})
	}
	var failures []error
	for attempt := range s.dialAttempts {
		if err := ctx.Err(); err != nil {
			return nil, errors.Join(append(failures, err)...)
		}
		backend := s.Pool.next(attempted)
		if backend == nil {
			if len(failures) == 0 {
				return nil, errors.New("no healthy backend available")
			}
			break
		}
		if attempted != nil {
			attempted[backend.Addr] = struct{}{}
		}
		conn, err := s.dialContext(ctx, "tcp", backend.Addr)
		if err == nil {
			return conn, nil
		}
		if conn != nil {
			_ = conn.Close()
		}
		failures = append(failures, fmt.Errorf("dial backend %s: %w", backend.Addr, err))
		if attempt+1 < s.dialAttempts {
			log.Warnf("backend dial failed (attempt %d/%d): %v", attempt+1, s.dialAttempts, failures[len(failures)-1])
		}
	}
	return nil, errors.Join(failures...)
}
