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
	"golang.org/x/net/context"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/txthinking/socks5"
)

type BackendCheckConfig struct {
	CheckURL     string `yaml:"check_url"`
	InitialAlive bool   `yaml:"initial_alive"`
	Timeout      uint   `yaml:"timeout"`
}

type Backend struct {
	Addr           string              `yaml:"addr"`
	Socks5UserName string              `yaml:"username"`
	Socks5Password string              `yaml:"password"`
	CheckConfig    *BackendCheckConfig `yaml:"check_config"`

	mux   sync.RWMutex
	alive bool
}

// Alive returns backend status
func (b *Backend) Alive() bool {
	return b.alive
}

// Check function to check the node healthy by given url
func (b *Backend) Check() error {
	b.mux.Lock()
	defer b.mux.Unlock()

	if url := b.CheckConfig.CheckURL; url != "" {
		client, err := b.httpProxyClient()
		if err != nil {
			return err
		}

		resp, err := client.Get(url)
		if err != nil || (resp != nil && resp.StatusCode != http.StatusOK) {
			b.alive = false
			return err
		}
	}

	b.alive = true
	return nil
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
	return socks5.NewClient(b.Addr, b.Socks5UserName, b.Socks5Password, timeout, timeout)
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
		CheckConfig: &config,
	}

	return
}
