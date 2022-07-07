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

type Configure struct {
	Socks5Listen string    `yaml:"socks5_listen"`
	TproxyListen string    `yaml:"tproxy_listen"`
	Backends     []Backend `yaml:"backends"`
}
