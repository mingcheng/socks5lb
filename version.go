/*!*
 * Copyright (c) 2025 Hangzhou Guanwaii Technology Co., Ltd.
 *
 * This source code is licensed under the MIT License,
 * which is located in the LICENSE file in the source tree's root directory.
 *
 * File: version.go
 * Author: mingcheng (mingcheng@apache.org)
 * File Created: Tuesday, June 21st 2022, 6:03:26 pm
 *
 * Modified By: mingcheng (mingcheng@apache.org)
 * Last Modified: 2025-10-07 11:23:00
 */

package socks5lb

import (
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
)

const AppName = "socks5lb"

var (
	Version     = "n/a"
	BuildCommit = "n/a"
	BuildDate   = "n/a"

	DebugMode = debugModeFromEnv()
	StartTime = time.Now()
)

func debugModeFromEnv() bool {
	value := GetEnv("DEBUG", "false")
	mode, err := strconv.ParseBool(value)
	if err != nil {
		log.Warnf("invalid DEBUG value %q: %v, disabling debug mode", value, err)
		return false
	}
	return mode
}
