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

// ServerConfig holds the configuration for all server components
type ServerConfig struct {
	// HTTP admin interface configuration
	HTTP struct {
		Addr string `yaml:"addr"`
	} `yaml:"http"`

	// TProxy transparent proxy configuration (not yet implemented)
	TProxy struct {
		Addr string `yaml:"addr"`
	} `yaml:"tproxy"`

	// Sock5 SOCKS5 proxy configuration
	Sock5 struct {
		Addr string `yaml:"addr"`
	} `yaml:"socks5"`
}

// Configure represents the complete application configuration
type Configure struct {
	ServerConfig ServerConfig `yaml:"server"`
	Backends     []Backend    `yaml:"backends"`
}
