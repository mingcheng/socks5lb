package socks5lb

import (
	"strconv"
	"time"
)

const AppName = "socks5lb"

var (
	Version     = "n/a"
	BuildCommit = "n/a"
	BuildDate   = "n/a"

	DebugMode = false
	StartTime time.Time
)

// init function for storage some runtime information
func init() {
	mode := GetEnv("DEBUG", "")
	DebugMode, _ = strconv.ParseBool(mode)

	// markup start time
	StartTime = time.Now()
}
