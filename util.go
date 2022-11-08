/**
 * File: util.go
 * Author: Ming Cheng<mingcheng@outlook.com>
 *
 * Created Date: Thursday, June 23rd 2022, 8:41:25 pm
 * Last Modified: Thursday, July 7th 2022, 6:31:41 pm
 *
 * http://www.opensource.org/licenses/MIT
 */

package socks5lb

import (
	log "github.com/sirupsen/logrus"
	"os"
	"strconv"
	"strings"
	"time"
)

// GetEnv to get the environment variables from system is not provided return default values
func GetEnv(name, def string) string {
	result := os.Getenv(name)
	if result == "" {
		result = def
	}

	return strings.TrimSpace(result)
}

// DurationFromEnv to get the seconds duration from system environment
func DurationFromEnv(name string, defVal uint64) time.Duration {
	str := GetEnv(name, strconv.FormatUint(defVal, 10))
	interval, err := strconv.ParseUint(str, 10, 64)
	if err != nil {
		log.Debugf("invalid interval %v, reset to 1s", err)
		interval = defVal
	}

	return time.Duration(interval) * time.Second
}

func ParseDuration(duration string, def time.Duration) time.Duration {
	d, err := time.ParseDuration(duration)
	if err != nil || d <= 0 {
		return def
	}

	return d
}
