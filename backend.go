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
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"net"
	"net/http"
	"time"

	"github.com/txthinking/socks5"
)

type BackendCheckConfig struct {
	CheckURL     string `yaml:"check_url" json:"check_url"`
	InitialAlive bool   `yaml:"initial_alive" json:"initial_alive"`
	Timeout      uint   `yaml:"timeout" json:"timeout"`
}

type Backend struct {
	Addr        string             `yaml:"addr" json:"addr" binding:"required"`
	UserName    string             `yaml:"username" json:"username"`
	Password    string             `yaml:"password" json:"password"`
	CheckConfig BackendCheckConfig `yaml:"check_config" json:"check_config"`

	alive bool
}

// Alive returns backend status
func (b *Backend) Alive() bool {
	return b.alive
}

// Check function to check the node healthy by given url
func (b *Backend) Check() (err error) {
	if url := b.CheckConfig.CheckURL; url != "" {
		var (
			client *http.Client
			resp   *http.Response
		)

		if client, err = b.httpProxyClient(); err != nil {
			return
		}

		resp, err = client.Head(url)
		if err != nil || (resp != nil && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusMovedPermanently && resp.StatusCode != http.StatusFound) {
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
			return b.Socks5Conn("tcp", addr, int(timeout))
		},
	}

	return &http.Client{
		Transport: httpTransport,
		Timeout:   time.Duration(timeout) * time.Second,
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

// NewBackend creates a new Backend instance
func NewBackend(addr string, config BackendCheckConfig) (backend *Backend) {
	backend = &Backend{
		Addr:        addr,
		alive:       config.InitialAlive,
		CheckConfig: config,
	}

	return
}
