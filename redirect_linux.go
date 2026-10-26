//go:build linux

/*!*
 * Copyright (c) 2025 Hangzhou Guanwaii Technology Co., Ltd.
 *
 * This source code is licensed under the MIT License,
 * which is located in the LICENSE file in the source tree's root directory.
 *
 * File: redirect_linux.go
 * Author: mingcheng (mingcheng@apache.org)
 * File Created: Wednesday, July 6th 2022, 11:47:00 am
 *
 * Modified By: mingcheng (mingcheng@apache.org)
 * Last Modified: 2025-10-07 11:24:03
 */

package socks5lb

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
)

// getOriginalDstAddr to get the original address from the socket
// this function is referenced from
// https://github.com/ginuerzh/gost/blob/0247b941ac31344f0d7b3c547941a051188ba202/redirect.go#L72
func getOriginalDstAddr(conn *net.TCPConn) (net.Addr, *net.TCPConn, error) {
	raw, err := conn.SyscallConn()
	if err != nil {
		return nil, nil, err
	}
	var mreq *syscall.IPv6Mreq
	var socketErr error
	if err := raw.Control(func(fd uintptr) {
		mreq, socketErr = syscall.GetsockoptIPv6Mreq(int(fd), syscall.IPPROTO_IP, 80)
	}); err != nil {
		return nil, nil, err
	}
	if socketErr != nil {
		return nil, nil, socketErr
	}

	// only ipv4 support
	ip := net.IPv4(mreq.Multiaddr[4], mreq.Multiaddr[5], mreq.Multiaddr[6], mreq.Multiaddr[7])
	port := uint16(mreq.Multiaddr[2])<<8 + uint16(mreq.Multiaddr[3])
	return &net.TCPAddr{IP: ip, Port: int(port)}, conn, nil
}

// ListenTProxy is listening the local tcp port on the given address
// Deprecated: this feature will be disabled in the future
func (s *Server) ListenTProxy(addr string) error {
	if s.Pool == nil {
		return fmt.Errorf("backend pool is required")
	}
	config := net.ListenConfig{
		Control: func(_, _ string, raw syscall.RawConn) error {
			var socketErr error
			if err := raw.Control(func(fd uintptr) {
				socketErr = syscall.SetsockoptInt(int(fd), syscall.SOL_IP, syscall.IP_TRANSPARENT, 1)
			}); err != nil {
				return err
			}
			return socketErr
		},
	}
	listener, err := config.Listen(s.connectionContext(), "tcp", addr)
	if err != nil {
		return err
	}
	if err := s.registerListener(listener, &s.tproxyListener); err != nil {
		_ = listener.Close()
		return err
	}

	selectTimeInterval := SecFromEnv("SELECT_TIME_INTERVAL", 300)
	log.Infof("auto select the socks5 proxy server every %v", selectTimeInterval)

	var selected atomic.Pointer[Backend]
	selectBackend := func() {
		backend := s.Pool.Next()
		selected.Store(backend)
		if backend == nil {
			log.Error("no healthy backend available for transparent proxy")
		}
	}
	selectBackend()
	ctx, cancel := context.WithCancel(s.connectionContext())
	done := make(chan struct{})
	defer func() {
		cancel()
		<-done
	}()
	go func() {
		defer close(done)
		timer := time.NewTicker(selectTimeInterval)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				selectBackend()
			}
		}
	}()

	return s.serveConnections(listener, func(conn net.Conn) {
		backend := selected.Load()
		if backend == nil {
			log.Error("no healthy backend available, closing transparent connection")
			return
		}
		connect, ok := conn.(*net.TCPConn)
		if !ok {
			log.Error("[red-tcp] not a TCP connection")
			return
		}

		dstAddr, originalConn, err := getOriginalDstAddr(connect)
		if err != nil {
			log.Errorf("[red-tcp] %s: %v", connect.RemoteAddr(), err)
			return
		}
		socks5Conn, err := backend.dialSocks5(ctx, "tcp", dstAddr.String(), DefaultDialTimeout)
		if err != nil {
			log.Errorf("dial transparent proxy backend %s: %v", backend.Addr, err)
			return
		}
		if !s.trackConnection(socks5Conn) {
			_ = socks5Conn.Close()
			return
		}
		defer s.releaseConnection(socks5Conn)
		if err := s.Transport(originalConn, socks5Conn); err != nil {
			log.Debugf("transparent transport error: %v", err)
		}
	})
}
