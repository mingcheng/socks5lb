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

import "fmt"

// ListenTProxy is not implemented by default
// Deprecated: this function is not implemented in next version
func (s *Server) ListenTProxy(_ string) error {
	return fmt.Errorf("sorry this feature is not implemented on this platform")
}
