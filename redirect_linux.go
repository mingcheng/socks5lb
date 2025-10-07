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
	"errors"
	"fmt"
	"github.com/LiamHaworth/go-tproxy"
	log "github.com/sirupsen/logrus"
	"github.com/txthinking/socks5"
	"net"
	"sync"
	"syscall"
	"time"
)

// getOriginalDstAddr to get the original address from the socket
// this function is referenced from
// https://github.com/ginuerzh/gost/blob/0247b941ac31344f0d7b3c547941a051188ba202/redirect.go#L72
func getOriginalDstAddr(conn *net.TCPConn) (addr net.Addr, c *net.TCPConn, err error) {
	defer conn.Close()

	fc, err := conn.File()
	if err != nil {
		return
	}
	defer fc.Close()

	mreq, err := syscall.GetsockoptIPv6Mreq(int(fc.Fd()), syscall.IPPROTO_IP, 80)
	if err != nil {
		return
	}

	// only ipv4 support
	ip := net.IPv4(mreq.Multiaddr[4], mreq.Multiaddr[5], mreq.Multiaddr[6], mreq.Multiaddr[7])
	port := uint16(mreq.Multiaddr[2])<<8 + uint16(mreq.Multiaddr[3])
	addr, err = net.ResolveTCPAddr("tcp4", fmt.Sprintf("%s:%d", ip.String(), port))
	if err != nil {
		return
	}

	cc, err := net.FileConn(fc)
	if err != nil {
		return
	}

	c, ok := cc.(*net.TCPConn)
	if !ok {
		err = errors.New("sorry, this is not a TCP connection")
	}

	return
}

var (
	currentSocks5Client *socks5.Client
	updateClientLock    sync.Mutex
)

// updateSocks5Client to connect to a proxy server
func (s *Server) updateSocks5Client() (client *socks5.Client, err error) {
	updateClientLock.Lock()

	defer func() {
		if err != nil {
			log.Error(err)
			return
		}

		if client != nil && currentSocks5Client != client {
			if currentSocks5Client != nil && (currentSocks5Client.TCPConn != nil || currentSocks5Client.UDPConn != nil) {
				log.Tracef("close pervious client before update the current socks5 client")
				_ = currentSocks5Client.Close()
				currentSocks5Client = nil
			}

			log.Infof("markup current socks5 proxy client %v", client.Server)
			currentSocks5Client = client
		}

		// lock the update client
		updateClientLock.Unlock()
	}()

	// found a available backend
	backend := s.Pool.Next()
	if backend != nil {
		return backend.socks5Client(0)
	}

	err = errors.New("sorry, we don't have healthy backend, so close the connection")
	return
}

// ListenTProxy is listening the local tcp port on the given address
// Deprecated: this feature will be disabled in the future
func (s *Server) ListenTProxy(addr string) (err error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		log.Error(err)
		return
	}

	s.tproxyListener, err = tproxy.ListenTCP("tcp", tcpAddr)
	if err != nil {
		log.Error(err)
		return
	}
	defer s.tproxyListener.Close()

	// connect to available socks5 proxy server
	selectTimeInterval := SecFromEnv("SELECT_TIME_INTERVAL", 300)
	log.Infof("auto select the socks5 proxy server every %v", selectTimeInterval)

	timer := time.NewTicker(selectTimeInterval)
	defer timer.Stop()
	go func() {
		for ; true; <-timer.C {
			log.Tracef("start to update current socks5 proxy client")
			if _, err := s.updateSocks5Client(); err != nil {
				log.Error(err)
			}
		}
	}()

	for {
		if s.tproxyListener == nil {
			return fmt.Errorf("transparent socket listenser is closed")
		}

		tproxyConn, err := s.tproxyListener.Accept()
		if err != nil {
			log.Error(err)
			continue
		}

		go func() {
			defer tproxyConn.Close()

			if currentSocks5Client == nil {
				log.Error("not found any suitable socks5 clients")
				return
			}
			log.Tracef("using connected socks5 proxy client: %v", currentSocks5Client.Server)

			connect, ok := tproxyConn.(*tproxy.Conn)
			if !ok {
				log.Error("[red-tcp] not a TCP connection")
				return
			}

			srcAddr := connect.RemoteAddr()
			dstAddr, orgDstConn, err := getOriginalDstAddr(connect.TCPConn)
			if err != nil {
				log.Errorf("[red-tcp] %s -> %s : %s", srcAddr, dstAddr, err)
				return
			}
			defer orgDstConn.Close()

			log.Tracef("[red-tcp] %s -> %s", srcAddr, dstAddr)
			socks5Conn, err := currentSocks5Client.Dial("tcp", dstAddr.String())
			if err != nil {
				log.Error(err)
			}
			defer socks5Conn.Close()

			s.Transport(orgDstConn, socks5Conn)
		}()
	}
}
