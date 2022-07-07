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
	"strconv"
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

type Server struct {
	Pool   *Pool
	Status map[*Backend]Status

	healthCheckTimer *time.Ticker
	socks5Listener   net.Listener
	tproxyListener   net.Listener
}

func (s *Server) AddBackend() error {
	return nil
}

func (s *Server) Start(socksListenAddr, tproxyListenAddr string) (err error) {
	intervalStr := GetEnv("CHECK_TIME_INTERVAL", "1")
	interval, err := strconv.ParseInt(intervalStr, 10, 64)
	if err != nil {
		log.Debugf("parse health check with error %v, reset default value 1", err)
		interval = 1
	}
	healthCheckTime := time.Duration(interval) * time.Second
	log.Tracef("check time interval is %v", healthCheckTime)

	s.healthCheckTimer = time.NewTicker(healthCheckTime)
	go func() {
		log.Tracef("start health check, every %v", healthCheckTime)
		for ; true; <-s.healthCheckTimer.C {
			s.Pool.Check()
		}
	}()

	if tproxyListenAddr != "" {
		log.Tracef("start linux transparent proxy on %s", tproxyListenAddr)
		go func() {
			if err = s.ListenTProxy(tproxyListenAddr); err != nil {
				log.Fatal(err)
			}
		}()
	}

	log.Tracef("start sock5 proxy address on %s", socksListenAddr)
	return s.ListenSocks5(socksListenAddr)
}

func (s *Server) Stop() (e error) {
	log.Debug("shutting down the server")
	s.healthCheckTimer.Stop()
	go s.socks5Listener.Close()
	go s.tproxyListener.Close()
	return
}

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
