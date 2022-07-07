//go:build linux

/**
 * File: redirect_linux.go
 * Author: Ming Cheng<mingcheng@outlook.com>
 *
 * Created Date: Wednesday, July 6th 2022, 11:47:00 am
 * Last Modified: Thursday, July 7th 2022, 6:32:42 pm
 *
 * http://www.opensource.org/licenses/MIT
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
	socks5Clients *socks5.Client
	clientLock    sync.Mutex
)

func (s *Server) socks5Client() (client *socks5.Client, err error) {
	clientLock.Lock()

	defer func() {
		clientLock.Unlock()
		if err != nil || client == nil {
			log.Error(err)
			socks5Clients = nil
			return
		}

		log.Infof("markup current proxy connection: %v", client.Server)
		socks5Clients = client
	}()

	backend := s.Pool.Next()
	if backend == nil {
		log.Error("sorry, we don't have healthy backend, so close the connection")
		socks5Clients = nil
		return
	}

	return backend.socks5Client(0)
}

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
	selectTimeInterval := SecFromEnv("SELECT_TIME_INTERVAL", 120)
	log.Infof("auto select the socks5 proxy server every %v", selectTimeInterval)

	timer := time.NewTicker(selectTimeInterval)
	defer timer.Stop()
	go func() {
		for ; true; <-timer.C {
			_, err := s.socks5Client()
			if err != nil {
				log.Error(err)
			}
		}
	}()

	for {
		tproxyConn, err := s.tproxyListener.Accept()
		if err != nil {
			log.Error(err)
			continue
		}

		go func() {
			defer tproxyConn.Close()

			if socks5Clients == nil {
				log.Error("not found any suitable socks5 clients")
				return
			}
			log.Tracef("using connected socks5 proxy client: %v", socks5Clients.Server)

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
			socks5Conn, err := socks5Clients.Dial("tcp", dstAddr.String())
			if err != nil {
				log.Error(err)
			}
			defer socks5Conn.Close()

			s.Transport(orgDstConn, socks5Conn)
		}()
	}
}
