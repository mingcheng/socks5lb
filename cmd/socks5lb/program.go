package main

import (
	"github.com/mingcheng/socks5lb"
	log "github.com/sirupsen/logrus"
)

import "github.com/judwhite/go-svc"

type program struct {
	Config *socks5lb.Configure
	pool   *socks5lb.Pool
	Server socks5lb.Server
}

func (p *program) Init(env svc.Environment) (err error) {

	log.Tracef("new initial backend pools")
	pool := socks5lb.NewPool()

	for _, v := range p.Config.Backends {
		log.Tracef("add backend %s", v.Addr)
		backend := socks5lb.NewBackend(v.Addr, *v.CheckConfig)
		pool.Add(backend)
	}

	p.Server = socks5lb.Server{
		Pool: pool,
	}

	return
}

func (p *program) Start() (err error) {
	go func() {
		err = p.Server.Start(p.Config.Socks5Listen, p.Config.TproxyListen)
	}()

	return
}

func (p *program) Stop() (err error) {
	return p.Server.Stop()
}
