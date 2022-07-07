/**
 * File: socks5.go
 * Author: Ming Cheng<mingcheng@outlook.com>
 *
 * Created Date: Wednesday, July 6th 2022, 11:46:39 am
 * Last Modified: Thursday, July 7th 2022, 6:31:37 pm
 *
 * http://www.opensource.org/licenses/MIT
 */

package socks5lb

import (
	log "github.com/sirupsen/logrus"
	"net"
)

// ListenSocks5 to listen on a specific address
func (s *Server) ListenSocks5(addr string) (err error) {
	s.socks5Listener, err = net.Listen("tcp", addr)
	if err != nil {
		log.Error(err)
		return
	}
	defer s.socks5Listener.Close()

	for {
		socks5Conn, err := s.socks5Listener.Accept()
		if err != nil {
			log.Error(err)
			continue
		}

		go func() {
			defer socks5Conn.Close()

			backend := s.Pool.Next()
			if backend == nil {
				log.Error("sorry, we don't have healthy backend, so close the connection")
				return
			}

			//log.Tracef("[socks5-tcp] %s -> %s", socks5Conn.RemoteAddr(), socks5Conn.LocalAddr())
			backendConn, err := net.Dial("tcp", backend.Addr)
			if err != nil {
				log.Error(err)
				return
			}
			defer backendConn.Close()

			s.Transport(socks5Conn, backendConn)
		}()
	}
}
