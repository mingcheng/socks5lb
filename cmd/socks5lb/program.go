/**
 * File: program.go
 * Author: Ming Cheng<mingcheng@outlook.com>
 *
 * Created Date: Wednesday, July 6th 2022, 2:14:35 pm
 * Last Modified: Thursday, July 7th 2022, 6:29:55 pm
 *
 * http://www.opensource.org/licenses/MIT
 */

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
