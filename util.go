/*!*
 * Copyright (c) 2025 Hangzhou Guanwaii Technology Co., Ltd.
 *
 * This source code is licensed under the MIT License,
 * which is located in the LICENSE file in the source tree's root directory.
 *
 * File: util.go
 * Author: mingcheng (mingcheng@apache.org)
 * File Created: Thursday, June 23rd 2022, 8:41:25 pm
 *
 * Modified By: mingcheng (mingcheng@apache.org)
 * Last Modified: 2025-10-07 11:21:02
 */

package socks5lb

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// GetEnv retrieves an environment variable value or returns a default if not set
// Trims whitespace from the result for cleaner configuration handling
func GetEnv(name, def string) string {
	result := strings.TrimSpace(os.Getenv(name))
	if result == "" {
		result = def
	}

	return strings.TrimSpace(result)
}

// SecFromEnv retrieves a duration in seconds from an environment variable
// Falls back to the default value if the variable is not set or invalid
func SecFromEnv(name string, defVal uint64) time.Duration {
	intervalStr := GetEnv(name, strconv.FormatUint(defVal, 10))
	interval, err := strconv.ParseUint(intervalStr, 10, 64)
	if err == nil {
		var duration time.Duration
		duration, err = secondsDuration(interval)
		if err == nil {
			return duration
		}
	}

	log.Warnf("invalid %s interval %q: %v, using default %ds", name, intervalStr, err, defVal)
	duration, err := secondsDuration(defVal)
	if err != nil {
		log.Warnf("invalid default interval %ds: %v, using 1s", defVal, err)
		return time.Second
	}
	return duration
}

func secondsDuration(seconds uint64) (time.Duration, error) {
	const maxSeconds = uint64((1<<63 - 1) / int64(time.Second))
	if seconds == 0 || seconds > maxSeconds {
		return 0, fmt.Errorf("interval must be between 1 and %d seconds", maxSeconds)
	}
	return time.Duration(seconds) * time.Second, nil
}
