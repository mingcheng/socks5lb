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

	// Select a healthy backend from the pool
	backend := s.Pool.Next()
	if backend == nil {
		log.Error("no healthy backend available, closing connection")
		return
	}

	// Dial the backend with timeout
	dialer := net.Dialer{
		Timeout:   DefaultDialTimeout,
		KeepAlive: DefaultKeepAlivePeriod,
	}

	backendConn, err := dialer.DialContext(s.connectionContext(), "tcp", backend.Addr)
	if err != nil {
		log.Errorf("failed to dial backend %s: %v", backend.Addr, err)
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
