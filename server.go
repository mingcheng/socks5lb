/**
 * File: server.go
 * Author: Ming Cheng<mingcheng@outlook.com>
 *
 * Created Date: Wednesday, July 6th 2022, 5:39:05 pm
 * Last Modified: Thursday, July 7th 2022, 6:31:24 pm
 *
 * http://www.opensource.org/licenses/MIT
 */

package socks5lb

import (
	"io"
	"net"
	"time"

	log "github.com/sirupsen/logrus"
)

// https://kasvith.me/posts/lets-create-a-simple-lb-go/

type Status struct {
	OutBytes, InBytes uint
	LastOnline        time.Time
	LastFailed        time.Time
	FailedTimes       uint
}

// Server that manages the proxy client
type Server struct {
	Pool   *Pool
	Config *ServerConfig

	socks5Listener net.Listener
	tproxyListener net.Listener
}

// AddBackend adds a backend to the pool
func (s *Server) AddBackend() error {
	// @TODO: need implemented
	return nil
}

// Start the proxy server and check
func (s *Server) Start() (err error) {

	// start goroutine for healthy check
	go s.Pool.Check()

	// start linux transport proxy if configure not empty
	if s.Config.TProxy.Addr != "" {
		log.Tracef("start linux transparent proxy on %s", s.Config.TProxy.Addr)
		go func() {
			if err = s.ListenTProxy(s.Config.TProxy.Addr); err != nil {
				log.Error(err)
			}
		}()
	}

	// start the http mirror server if configured
	if s.Config.HTTP.Addr != "" {
		log.Tracef("start http admin control on %s", s.Config.HTTP.Addr)
		go func() {
			if err = s.ListenHTTPAdmin(s.Config.HTTP.Addr); err != nil {
				log.Error(err)
			}
		}()
	}

	log.Tracef("start sock5 proxy address on %s", s.Config.Sock5.Addr)
	return s.ListenSocks5(s.Config.Sock5.Addr)
}

func (s *Server) Stop() (e error) {
	log.Debug("shutting down the server")
	if e = s.Pool.Close(); e != nil {
		return
	}

	if s.socks5Listener != nil {
		go s.socks5Listener.Close()
	}

	if s.tproxyListener != nil {
		go s.tproxyListener.Close()
	}

	return
}

// Transport is used to connect to the server and client each	other
func (s *Server) Transport(dst, src io.ReadWriter) (err error) {
	// @see https://github.com/ginuerzh/gost/blob/0247b941ac31344f0d7b3c547941a051188ba202/server.go#L105
	errs := make(chan error, 1)

	go func() {
		_, err = io.Copy(dst, src)
		errs <- err
	}()

	go func() {
		_, err = io.Copy(src, dst)
		errs <- err
	}()

	err = <-errs
	if err != nil && err == io.EOF {
		err = nil
	}

	log.Tracef("transport stream is finished")
	return
}

func NewServer(pool *Pool, config ServerConfig) (*Server, error) {
	return &Server{
		Pool:   pool,
		Config: &config,
	}, nil
}
