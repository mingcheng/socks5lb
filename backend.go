/**
 * File: backend.go
 * Author: Ming Cheng<mingcheng@outlook.com>
 *
 * Created Date: Tuesday, June 21st 2022, 6:03:26 pm
 * Last Modified: Thursday, July 7th 2022, 6:30:08 pm
 *
 * http://www.opensource.org/licenses/MIT
 */

package socks5lb

import (
	"context"
	log "github.com/sirupsen/logrus"
	"github.com/txthinking/socks5"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type BackendCheckConfig struct {
	CheckURL     string `yaml:"check_url" json:"check_url"`
	InitialAlive bool   `yaml:"initial_alive" json:"initial_alive"`
	Timeout      string `yaml:"timeout" json:"timeout"`
	Period       string `yaml:"period" json:"period"`
}

type Backend struct {
	Addr        string             `yaml:"addr" json:"addr" binding:"required"`
	UserName    string             `yaml:"username" json:"username"`
	Password    string             `yaml:"password" json:"password"`
	CheckConfig BackendCheckConfig `yaml:"check_config" json:"check_config"`

	alive  bool
	ticker *time.Ticker
	mutex  sync.Mutex
}

// Alive returns backend status
func (b *Backend) Alive() bool {
	return b.alive
}

// PeriodCheck to check if backend is healthy periodically
func (b *Backend) PeriodCheck() (err error) {
	period := ParseDuration(b.CheckConfig.Period, DurationFromEnv("CHECK_INTERVAL", 60))
	log.Debugf("period check for backend %s is %d", b.Addr, period)

	b.ticker = time.NewTicker(period)
	log.Infof("auto check backend healthy, every %v", b.ticker)

	for ; true; <-b.ticker.C {
		b.Check()
	}

	return nil
}

// Check function to check the node healthy by given url
func (b *Backend) Check() (err error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if url := b.CheckConfig.CheckURL; url != "" {
		var (
			client *http.Client
			resp   *http.Response
		)

		if client, err = b.httpProxyClient(); err != nil {
			return
		}

		resp, err = client.Get(url)
		if err != nil || (resp != nil && resp.StatusCode != http.StatusOK) {
			log.Error(err)
			b.alive = false
		} else {
			b.alive = true
		}

		return
	}

	b.alive = b.CheckConfig.InitialAlive
	return
}

// httpProxyClient to create http client with socks5 proxy
func (b *Backend) httpProxyClient() (*http.Client, error) {
	var timeout = b.CheckConfig.Timeout

	// setup a http client
	httpTransport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			timeoutInt, _ := strconv.ParseInt(timeout, 10, 64)
			log.Tracef("the timeout int value is %v", timeoutInt)
			return b.Socks5Conn("tcp", addr, int(timeoutInt))
		},
	}

	d := ParseDuration(timeout, DurationFromEnv("CHECK_TIMEOUT", 10))
	log.Tracef("timeout is %v", d)

	return &http.Client{
		Transport: httpTransport,
		Timeout:   d,
	}, nil
}

// socks5Client to create http client with socks5 proxy
func (b *Backend) socks5Client(timeout int) (*socks5.Client, error) {
	return socks5.NewClient(string(b.Addr), b.UserName, b.Password, timeout, timeout)
}

// Socks5Conn to create a connection by specific params
func (b *Backend) Socks5Conn(network, addr string, timeout int) (cc net.Conn, err error) {
	client, err := b.socks5Client(timeout)
	if err != nil {
		return
	}

	return client.Dial(network, addr)
}

// StopCheck to stop health check
func (b *Backend) StopCheck() error {
	if b.ticker != nil {
		b.ticker.Stop()
		b.ticker = nil
	}

	return nil
}

// NewBackend creates a new Backend instance
func NewBackend(addr string, config BackendCheckConfig) (backend *Backend) {
	backend = &Backend{
		Addr:        addr,
		alive:       config.InitialAlive,
		CheckConfig: config,
	}

	return
}
