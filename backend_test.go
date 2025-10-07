/*!*
 * Copyright (c) 2025 Hangzhou Guanwaii Technology Co., Ltd.
 *
 * This source code is licensed under the MIT License,
 * which is located in the LICENSE file in the source tree's root directory.
 *
 * File: backend_test.go
 * Author: mingcheng (mingcheng@apache.org)
 * File Created: 2025-10-07 11:08:41
 *
 * Modified By: mingcheng (mingcheng@apache.org)
 * Last Modified: 2025-10-07 11:23:12
 */

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
