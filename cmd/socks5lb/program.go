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
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/judwhite/go-svc"
	"github.com/mingcheng/socks5lb"
	log "github.com/sirupsen/logrus"
)

// program to run a specific version of the local package socks5lb
type program struct {
	Config *socks5lb.Configure
	Server *socks5lb.Server

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	runErr error
}

// Init to initial the program
func (p *program) Init(svc.Environment) error {
	if p.Config == nil {
		return errors.New("configuration is required")
	}

	log.Tracef("new initial backend pools")
	pool := socks5lb.NewPool()

	for _, v := range p.Config.Backends {
		log.Tracef("add backend %s", v.Addr)
		backend := v
		backend.SetAlive(backend.CheckConfig.InitialAlive)
		if err := pool.Add(&backend); err != nil {
			return fmt.Errorf("initialize backend pool: %w", err)
		}
	}

	server, err := socks5lb.NewServer(pool, p.Config.ServerConfig)
	if err != nil {
		return err
	}
	p.Server = server
	p.ctx, p.cancel = context.WithCancel(context.Background())
	return nil
}

// Start when the program is start
func (p *program) Start() error {
	if p.Server == nil || p.cancel == nil {
		return errors.New("program is not initialized")
	}
	if p.done != nil {
		return errors.New("program is already started")
	}
	p.done = make(chan struct{})
	log.Infof("start the program")
	go func() {
		defer close(p.done)
		defer p.cancel()
		p.runErr = p.Server.Start()
		if errors.Is(p.runErr, net.ErrClosed) && p.ctx.Err() != nil {
			p.runErr = nil
		}
		if p.runErr != nil {
			log.Error(p.runErr)
		}
	}()
	return nil
}

// Stop when the program is stopped
func (p *program) Stop() error {
	log.Infof("stop the program")
	if p.cancel != nil {
		p.cancel()
	}
	if p.Server == nil {
		return nil
	}
	stopErr := p.Server.Stop()
	if p.done != nil {
		<-p.done
	}
	return errors.Join(stopErr, p.runErr)
}

// Context lets the service runner exit when the server fails.
func (p *program) Context() context.Context {
	return p.ctx
}
