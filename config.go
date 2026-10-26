/*!*
 * Copyright (c) 2025 Hangzhou Guanwaii Technology Co., Ltd.
 *
 * This source code is licensed under the MIT License,
 * which is located in the LICENSE file in the source tree's root directory.
 *
 * File: config.go
 * Author: mingcheng (mingcheng@apache.org)
 * File Created: Tuesday, June 21st 2022, 6:03:38 pm
 *
 * Modified By: mingcheng (mingcheng@apache.org)
 * Last Modified: 2025-10-07 11:21:34
 */

package socks5lb

import "fmt"

// ConnectionConfig controls proxy admission and backend dialing.
// Zero limits are unlimited; zero attempts and timeout use the existing defaults.
type ConnectionConfig struct {
	MaxConnections     int  `yaml:"max_connections"`
	MaxConcurrentDials int  `yaml:"max_concurrent_dials"`
	DialAttempts       int  `yaml:"dial_attempts"`
	DialTimeout        uint `yaml:"dial_timeout"`
}

// HealthCheckConfig controls server-managed checks, independently of Backend.Check.
// Zero concurrency and thresholds mean one; startup checks and jitter are opt-in.
type HealthCheckConfig struct {
	Concurrency      int  `yaml:"concurrency"`
	CheckOnStart     bool `yaml:"check_on_start"`
	JitterPercent    int  `yaml:"jitter_percent"`
	FailureThreshold int  `yaml:"failure_threshold"`
	SuccessThreshold int  `yaml:"success_threshold"`
}

// ServerConfig holds the configuration for all server components
type ServerConfig struct {
	// HTTP admin interface configuration
	HTTP struct {
		Addr string `yaml:"addr"`
	} `yaml:"http"`

	// Legacy Linux transparent proxy; Server.Start does not enable this listener.
	TProxy struct {
		Addr string `yaml:"addr"`
	} `yaml:"tproxy"`

	// Sock5 SOCKS5 proxy configuration
	Sock5 struct {
		Addr string `yaml:"addr"`
	} `yaml:"socks5"`

	Connections ConnectionConfig  `yaml:"connections"`
	HealthCheck HealthCheckConfig `yaml:"health_check"`
}

// Validate checks optional settings. Start separately requires a SOCKS5 address.
func (c ServerConfig) Validate() error {
	for _, setting := range []struct {
		name  string
		value int
	}{
		{"connections.max_connections", c.Connections.MaxConnections},
		{"connections.max_concurrent_dials", c.Connections.MaxConcurrentDials},
		{"connections.dial_attempts", c.Connections.DialAttempts},
		{"health_check.concurrency", c.HealthCheck.Concurrency},
		{"health_check.failure_threshold", c.HealthCheck.FailureThreshold},
		{"health_check.success_threshold", c.HealthCheck.SuccessThreshold},
	} {
		if setting.value < 0 {
			return fmt.Errorf("server.%s must not be negative", setting.name)
		}
	}
	if c.HealthCheck.JitterPercent < 0 || c.HealthCheck.JitterPercent > 100 {
		return fmt.Errorf("server.health_check.jitter_percent must be between 0 and 100")
	}
	if timeout := c.Connections.DialTimeout; timeout != 0 {
		if _, err := secondsDuration(uint64(timeout)); err != nil {
			return fmt.Errorf("server.connections.dial_timeout: %w", err)
		}
	}
	return nil
}

// Configure represents the complete application configuration
type Configure struct {
	ServerConfig ServerConfig `yaml:"server"`
	Backends     []Backend    `yaml:"backends"`
}
