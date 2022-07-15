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

func init() {
	mode := GetEnv("DEBUG", "")
	DebugMode, _ = strconv.ParseBool(mode)

	// markup start time
	StartTime = time.Now()
}
