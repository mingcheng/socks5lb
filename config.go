package socks5lb

type Configure struct {
	Listen   string   `yaml:"listen"`
	Status   string   `yaml:"status"`
	Backends []string `yaml:"backends"`
}
