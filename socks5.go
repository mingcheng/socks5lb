package socks5lb

import (
	log "github.com/sirupsen/logrus"
	"net"
)

func (s *Server) ListenSocks5(addr string) (err error) {
	s.socks5Listener, err = net.Listen("tcp", addr)
	if err != nil {
		log.Error(err)
		return
	}

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
