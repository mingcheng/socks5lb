package socks5lb

type Configure struct {
	Socks5Listen string    `yaml:"socks5_listen"`
	TproxyListen string    `yaml:"tproxy_listen"`
	Backends     []Backend `yaml:"backends"`
}
