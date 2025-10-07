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
func (s *Server) ListenSocks5(addr string) (err error) {
	s.socks5Listener, err = net.Listen("tcp", addr)
	if err != nil {
		log.Error(err)
		return
	}
	defer s.socks5Listener.Close()

	for {
		var socks5Conn net.Conn
		socks5Conn, err = s.socks5Listener.Accept()
		if err != nil {
			log.Error(err)
			return
		}

		// Handle each connection in a separate goroutine
		go s.handleSocks5Connection(socks5Conn)
	}
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

	backendConn, err := dialer.Dial("tcp", string(backend.Addr))
	if err != nil {
		log.Errorf("failed to dial backend %s: %v", backend.Addr, err)
		return
	}
	defer backendConn.Close()

	// Enable TCP keepalive on backend connection
	if tcpConn, ok := backendConn.(*net.TCPConn); ok {
		if err := tcpConn.SetKeepAlive(true); err != nil {
			log.Warnf("failed to set backend keepalive: %v", err)
		}
		if err := tcpConn.SetKeepAlivePeriod(DefaultKeepAlivePeriod); err != nil {
			log.Warnf("failed to set backend keepalive period: %v", err)
		}
	}

	// Transport data bidirectionally between client and backend
	if err := s.Transport(socks5Conn, backendConn); err != nil {
		log.Debugf("transport error: %v", err)
	}
}
