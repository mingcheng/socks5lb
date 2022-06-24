package socks5lb

import (
	"os"
	"strings"
)

func GetEnv(name, def string) string {
	result := os.Getenv(name)
	if result == "" {
		result = def
	}

	return strings.TrimSpace(result)
}
