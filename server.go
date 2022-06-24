package socks5lb

import (
	log "github.com/sirupsen/logrus"
	"io"
	"net"
	"strconv"
	"time"
)

type Server struct {
	Pool             *Pool
	healthCheckTimer *time.Ticker
	ln               net.Listener
}

func (s *Server) AddBackend() error {
	return nil
}

func (s *Server) Start(addr string) error {
	var err error

	intervalStr := GetEnv("CHECK_TIME_INTERVAL", "1")
	interval, err := strconv.ParseInt(intervalStr, 10, 64)
	if err != nil {
		interval = 1
	}

	healthCheckTime := time.Duration(interval) * time.Minute
	//log.Tracef("check time interval is %v", healthCheckTime)

	s.healthCheckTimer = time.NewTicker(healthCheckTime)

	go func() {
		log.Tracef("start health check, every %v", healthCheckTime)
		for ; true; <-s.healthCheckTimer.C {
			s.Pool.HealthCheck("https://www.google.com/robots.txt")
		}
	}()

	log.Debugf("start listen on %s", addr)
	s.ln, err = net.Listen("tcp", addr)
	if err != nil {
		log.Error(err)
		return err
	}

	for {
		conn, err := s.ln.Accept()
		if err != nil {
			log.Error(err)
			continue
		}

		err = conn.SetDeadline(time.Now().Add(time.Minute * 30))
		if err != nil {
			log.Error(err)
			continue
		}

		log.Tracef("new tcp connection from %s", conn.RemoteAddr())
		go func() {
			err := s.handleConnection(conn, s.Pool.Next())
			if err != nil {
				log.Error(err)
			}
		}()
	}
}

func (s *Server) Stop() error {
	log.Debug("shutting down the server")
	s.healthCheckTimer.Stop()
	return s.ln.Close()
}

func (s *Server) copy(dst io.Writer, src io.Reader) error {
	count, err := io.Copy(dst, src)
	log.Tracef("%d bytes copied", count)
	return err
}

func (s *Server) handleConnection(us net.Conn, server *Backend) error {
	if server == nil {
		return nil
	}

	ds, err := net.DialTimeout("tcp", server.Addr, 3*time.Second)
	if err != nil {
		return err
	}

	log.Tracef("%v >-< %v", us.RemoteAddr(), ds.LocalAddr())
	errc := make(chan error, 1)

	go func() {
		errc <- s.copy(ds, us)
	}()

	go func() {
		errc <- s.copy(us, ds)
	}()

	err = <-errc
	if err != nil && err == io.EOF {
		err = nil
	}

	return err
}
