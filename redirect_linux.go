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
	"net"
	"syscall"
)

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
		err = errors.New("not a TCP connection")
	}

	return
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

	for {
		tproxyConn, err := s.tproxyListener.Accept()
		if err != nil {
			log.Error(err)
			continue
		}

		go func() {
			defer tproxyConn.Close()

			backend := s.Pool.Next()
			if backend == nil {
				log.Error("sorry, we don't have healthy backend, so close the connection")
				return
			}

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
			socks5Conn, err := backend.Socks5Conn("tcp", dstAddr.String(), 0)
			if err != nil {
				log.Error(err)
			}
			defer socks5Conn.Close()

			s.Transport(orgDstConn, socks5Conn)
		}()
	}
}
