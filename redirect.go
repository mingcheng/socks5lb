//go:build !linux

package socks5lb

import (
	"fmt"
)

func (s *Server) ListenTProxy(_ string) (err error) {
	err = fmt.Errorf("sorry transparent proxy is ONLY supports Linux platform")
	return
}
