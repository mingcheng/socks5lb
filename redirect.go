//go:build !linux

/*!*
 * Copyright (c) 2025 Hangzhou Guanwaii Technology Co., Ltd.
 *
 * This source code is licensed under the MIT License,
 * which is located in the LICENSE file in the source tree's root directory.
 *
 * File: redirect.go
 * Author: mingcheng (mingcheng@apache.org)
 * File Created: Wednesday, July 6th 2022, 11:46:51 am
 *
 * Modified By: mingcheng (mingcheng@apache.org)
 * Last Modified: 2025-10-07 11:23:46
 */

package socks5lb

import "fmt"

// ListenTProxy is not implemented by default
// Deprecated: this function is not implemented in next version
func (s *Server) ListenTProxy(_ string) error {
	return fmt.Errorf("sorry this feature is not implemented on this platform")
}
