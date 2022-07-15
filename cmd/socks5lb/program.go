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

// program to run a specific version of the local package socks5lb
type program struct {
	Config *socks5lb.Configure
	Server *socks5lb.Server
}

// Init to initial the program
func (p *program) Init(env svc.Environment) (err error) {

	log.Tracef("new initial backend pools")
	pool := socks5lb.NewPool()

	for _, v := range p.Config.Backends {
		log.Tracef("add backend %s", v.Addr)
		backend := socks5lb.NewBackend(v.Addr, v.CheckConfig)
		_ = pool.Add(backend)
	}

	p.Server, err = socks5lb.NewServer(pool, p.Config.ServerConfig)

	return
}

// Start when the program is start
func (p *program) Start() (err error) {
	log.Infof("start the program")
	go func() {
		if err = p.Server.Start(); err != nil {
			log.Error(err)
			p.Server.Stop()
		}
	}()

	return
}

// Stop when the program is stop
func (p *program) Stop() (err error) {
	log.Infof("stop the program")
	return p.Server.Stop()
}
