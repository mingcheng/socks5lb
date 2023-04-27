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

// init function to initialize the global variables
func init() {
	mode := GetEnv("DEBUG", "")
	DebugMode, _ = strconv.ParseBool(mode)

	// mark the start time
	StartTime = time.Now()
}
