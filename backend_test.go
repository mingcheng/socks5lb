package socks5lb

import (
	"testing"
)

func TestBackend_Check(t *testing.T) {
	b := NewBackend("192.168.100.254:1086", BackendCheckConfig{
		CheckURL:     "https://www.google.com/robots.txt",
		InitialAlive: true,
	})

	err := b.Check()
	if err != nil {
		t.Error(err)
	}
}
