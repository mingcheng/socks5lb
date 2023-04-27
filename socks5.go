/**
 * File: socks5.go
 * Author: Ming Cheng<mingcheng@outlook.com>
 *
 * Created Date: Wednesday, July 6th 2022, 11:46:39 am
 * Last Modified: Thursday, February 16th 2023, 3:37:10 pm
 *
 * http://www.opensource.org/licenses/MIT
 */

package socks5lb

import (
	log "github.com/sirupsen/logrus"
	"net"
)

// ListenSocks5 is used to listen the socks5 proxy port and forward the connection to the backend
func (s *Server) ListenSocks5(addr string) (err error) {
	s.socks5Listener, err = net.Listen("tcp", addr)

	// if the address is already in use, abort the function and return the error
	if err != nil {
		log.Error(err)
		return
	}
	defer s.socks5Listener.Close()

	// start the socks5 server and listen the port
	for {
		var socks5Conn net.Conn
		socks5Conn, err = s.socks5Listener.Accept()
		if err != nil {
			log.Error(err)
			return
		}

		go func() {
			defer socks5Conn.Close()

			backend := s.Pool.Next()
			if backend == nil {
				log.Error("sorry, we don't have healthy backend, so close the connection")
				return
			}

			//log.Tracef("[socks5-tcp] %s -> %s", socks5Conn.RemoteAddr(), socks5Conn.LocalAddr())
			backendConn, err := net.Dial("tcp", string(backend.Addr))
			if err != nil {
				log.Error(err)
				return
			}
			defer backendConn.Close()

			// transport the socket connection directly to the backend
			s.Transport(socks5Conn, backendConn)
		}()
	}
}
