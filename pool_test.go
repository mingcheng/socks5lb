package socks5lb

import (
	"fmt"
	"testing"
)

func TestPool_HealthCheck(t *testing.T) {

	pool := Pool{
		backends: []*Backend{
			{
				Addr: "10.0.20.254:1086",
			},
			{
				Addr: "10.0.20.254:1086",
			},
			{
				Addr: "10.0.11.254:1086",
			},
			{
				Addr: "10.0.11.254:1086",
			},
			{
				Addr: "192.168.1.254:1086",
			},
			{
				Addr: "172.16.1.254:1086",
			},
		},
	}

	pool.HealthCheck("https://www.google.com/robots.txt")

	for i := 0; i < 100; i++ {
		b := pool.Next()
		if b != nil {
			fmt.Printf("%v | ", pool.current)
			fmt.Printf("%v\n", b)
		}
	}
}
