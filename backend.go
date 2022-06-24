package socks5lb

import (
	"golang.org/x/net/context"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/txthinking/socks5"
)

type Backend struct {
	Addr  string `yaml:"addr"`
	mux   sync.RWMutex
	alive bool
}

// Alive returns backend status
func (b *Backend) Alive() bool {
	return b.alive
}

// Check function to check the node healthy by given url
func (b *Backend) Check(url string) error {
	b.mux.Lock()
	defer b.mux.Unlock()

	client, err := b.proxyClient()
	if err != nil {
		return err
	}

	resp, err := client.Get(url)
	if err != nil || (resp != nil && resp.StatusCode != http.StatusOK) {
		b.alive = false
		return err
	}

	b.alive = true
	return nil
}

// proxyClient to create http client with socks5 proxy
func (b *Backend) proxyClient() (*http.Client, error) {
	// NOTICE timeout as seconds
	timeout := 30
	c, err := socks5.NewClient(b.Addr, "", "", timeout, timeout)
	if err != nil {
		return nil, err
	}

	// setup a http client
	httpTransport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return c.Dial(network, addr)
		},
	}

	return &http.Client{
		Transport: httpTransport,
		Timeout:   time.Duration(timeout) * time.Second,
	}, nil
}
