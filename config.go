/**
 * File: config.go
 * Author: Ming Cheng<mingcheng@outlook.com>
 *
 * Created Date: Tuesday, June 21st 2022, 6:03:38 pm
 * Last Modified: Thursday, July 7th 2022, 6:30:15 pm
 *
 * http://www.opensource.org/licenses/MIT
 */

package socks5lb

type ServerConfig struct {
	HTTP struct {
		Addr string `yaml:"addr"`
	} `yaml:"http"`

	TProxy struct {
		Addr string `yaml:"addr"`
	} `yaml:"tproxy"`

	Sock5 struct {
		Addr string `yaml:"addr"`
	} `yaml:"sock5"`
}

type Configure struct {
	ServerConfig *ServerConfig `yaml:"server"`
	Backends     []Backend     `yaml:"backends"`
}
