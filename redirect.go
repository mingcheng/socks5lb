//go:build !linux

/**
 * File: redirect.go
 * Author: Ming Cheng<mingcheng@outlook.com>
 *
 * Created Date: Wednesday, July 6th 2022, 11:46:51 am
 * Last Modified: Thursday, July 7th 2022, 6:31:04 pm
 *
 * http://www.opensource.org/licenses/MIT
 */

package socks5lb

// ListenProxy is not implemented by default
func (s *Server) ListenTProxy(_ string) error {
	return error.New("sorry transparent proxy is ONLY supports Linux platform")
}
