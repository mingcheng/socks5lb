package socks5lb

import (
	"testing"
)

func TestBackend_Check(t *testing.T) {
	b := Backend{
		Addr: "10.0.20.25:1086",
	}

	err := b.Check("https://www.google.com/robots.txt")
	if err != nil {
		t.Error(err)
	}
}
