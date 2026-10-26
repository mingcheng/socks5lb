/*!*
 * Copyright (c) 2025 Hangzhou Guanwaii Technology Co., Ltd.
 *
 * This source code is licensed under the MIT License,
 * which is located in the LICENSE file in the source tree's root directory.
 *
 * File: server.go
 * Author: mingcheng (mingcheng@apache.org)
 * File Created: Wednesday, July 6th 2022, 5:39:05 pm
 *
 * Modified By: mingcheng (mingcheng@apache.org)
 * Last Modified: 2025-10-07 11:22:20
 */

package socks5lb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

const (
	// BufferSize defines the size of buffers used for copying data between connections
	BufferSize = 32 * 1024 // 32KB buffer for better performance
)

var (
	// bufferPool reuses buffers to reduce GC pressure
	bufferPool = &sync.Pool{
		New: func() any {
			return new([BufferSize]byte)
		},
	}
)

// https://kasvith.me/posts/lets-create-a-simple-lb-go/

// Status tracks statistics and health information for connections
type Status struct {
	OutBytes, InBytes uint
	LastOnline        time.Time
	LastFailed        time.Time
	FailedTimes       uint
}

// Server represents the main SOCKS5 load balancer server.
// A Server must not be copied after first use.
type Server struct {
	Pool   *Pool
	Config *ServerConfig

	mu              sync.Mutex
	started         bool
	stopped         bool
	ctx             context.Context
	cancel          context.CancelFunc
	healthDone      chan struct{}
	stopDone        chan struct{}
	stopErr         error
	engine          *gin.Engine
	httpServer      *http.Server
	httpListener    net.Listener
	socks5Listener  net.Listener
	tproxyListener  net.Listener
	connections     map[net.Conn]struct{}
	connectionWG    sync.WaitGroup
	configured      bool
	connectionSlots chan struct{}
	dialSlots       chan struct{}
	dialTimeout     time.Duration
	dialAttempts    int
	dialContext     func(context.Context, string, string) (net.Conn, error)
}

// AddBackend is retained for compatibility.
// Deprecated: use Server.Pool.Add with a configured Backend instead.
func (s *Server) AddBackend() error {
	return errors.New("use Server.Pool.Add to add a backend")
}

// Start initializes and starts all server components
// - Health check timer for periodic backend monitoring
// - HTTP admin interface (if configured)
// - SOCKS5 proxy listener
func (s *Server) Start() (err error) {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return net.ErrClosed
	}
	if s.started {
		s.mu.Unlock()
		return errors.New("server is already started")
	}
	if s.Pool == nil || s.Config == nil || strings.TrimSpace(s.Config.Sock5.Addr) == "" {
		s.mu.Unlock()
		return errors.New("backend pool and SOCKS5 listen address are required")
	}
	s.started = true
	s.mu.Unlock()

	serveErrors := make(chan error, 2)
	serving := 0
	defer func() {
		err = errors.Join(err, s.Stop())
		for i := 1; i < serving; i++ {
			err = errors.Join(err, <-serveErrors)
		}
	}()

	listener, err := s.listenSocks5(s.Config.Sock5.Addr)
	if err != nil {
		return err
	}

	var admin *http.Server
	var adminListener net.Listener
	if s.Config.HTTP.Addr != "" {
		admin, adminListener, err = s.listenHTTPAdmin(s.Config.HTTP.Addr)
		if err != nil {
			return err
		}
	}

	s.startHealthChecks(SecFromEnv("CHECK_TIME_INTERVAL", 60))
	serving++
	go func() { serveErrors <- s.serveConnections(listener, s.handleSocks5Connection) }()
	if admin != nil {
		serving++
		go func() { serveErrors <- serveHTTPAdmin(admin, adminListener) }()
	}
	return <-serveErrors
}

func (s *Server) startHealthChecks(interval time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return
	}
	ctx := s.contextLocked()
	s.healthDone = make(chan struct{})
	done := s.healthDone
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.Pool.check(ctx)
			}
		}
	}()
}

func (s *Server) contextLocked() context.Context {
	if s.ctx == nil {
		s.ctx, s.cancel = context.WithCancel(context.Background())
		if s.stopped {
			s.cancel()
		}
	}
	return s.ctx
}

func (s *Server) connectionContext() context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.contextLocked()
}

func (s *Server) registerListener(listener net.Listener, target *net.Listener) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return net.ErrClosed
	}
	if *target != nil {
		return errors.New("listener is already running")
	}
	*target = listener
	return nil
}

func (s *Server) trackConnection(conn net.Conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return false
	}
	if s.connections == nil {
		s.connections = make(map[net.Conn]struct{})
	}
	s.connections[conn] = struct{}{}
	s.connectionWG.Add(1)
	return true
}

func (s *Server) releaseConnection(conn net.Conn) {
	_ = conn.Close()
	s.mu.Lock()
	delete(s.connections, conn)
	s.mu.Unlock()
	s.connectionWG.Done()
}

func (s *Server) serveConnections(listener net.Listener, handle func(net.Conn)) error {
	defer listener.Close()
	ctx := s.connectionContext()
	var retryDelay time.Duration
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			var timeout interface{ Timeout() bool }
			if isResourceAcceptError(err) || (errors.As(err, &timeout) && timeout.Timeout()) {
				if retryDelay == 0 {
					retryDelay = 5 * time.Millisecond
				} else {
					retryDelay = min(2*retryDelay, time.Second)
				}
				log.Warnf("accept failed, retrying in %s: %v", retryDelay, err)
				if err := waitContext(ctx, retryDelay); err != nil {
					return nil
				}
				continue
			}
			return fmt.Errorf("accept connection: %w", err)
		}
		retryDelay = 0
		if s.connectionSlots != nil {
			select {
			case s.connectionSlots <- struct{}{}:
			default:
				log.Warn("proxy connection limit reached, closing connection")
				_ = conn.Close()
				continue
			}
		}
		if !s.trackConnection(conn) {
			s.releaseConnectionSlot()
			_ = conn.Close()
			return nil
		}
		go func() {
			defer s.releaseConnection(conn)
			defer s.releaseConnectionSlot()
			handle(conn)
		}()
	}
}

func (s *Server) releaseConnectionSlot() {
	if s.connectionSlots != nil {
		<-s.connectionSlots
	}
}

func waitContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Stop gracefully shuts down the server and all listeners
func (s *Server) Stop() error {
	log.Debug("initiating server shutdown")
	s.mu.Lock()
	if s.stopDone != nil {
		done := s.stopDone
		s.mu.Unlock()
		<-done
		return s.stopErr
	}
	s.stopped = true
	s.stopDone = make(chan struct{})
	if s.cancel != nil {
		s.cancel()
	}
	listeners := []net.Listener{s.socks5Listener, s.tproxyListener, s.httpListener}
	admin := s.httpServer
	healthDone := s.healthDone
	connections := make([]net.Conn, 0, len(s.connections))
	for conn := range s.connections {
		connections = append(connections, conn)
	}
	s.mu.Unlock()

	var errs []error
	for _, listener := range listeners {
		if listener != nil {
			if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				errs = append(errs, err)
			}
		}
	}
	for _, conn := range connections {
		if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			errs = append(errs, err)
		}
	}
	if admin != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := admin.Shutdown(ctx); err != nil {
			errs = append(errs, err, admin.Close())
		}
	}
	if healthDone != nil {
		<-healthDone
	}
	s.connectionWG.Wait()
	s.mu.Lock()
	s.stopErr = errors.Join(errs...)
	close(s.stopDone)
	s.mu.Unlock()
	return s.stopErr
}

// Transport bidirectionally copies data between dst and src connections
// Uses buffer pooling to reduce memory allocations and GC pressure
func (s *Server) Transport(dst, src io.ReadWriter) (err error) {
	// @see https://github.com/ginuerzh/gost/blob/0247b941ac31344f0d7b3c547941a051188ba202/server.go#L105

	errs := make(chan error, 2)

	go copyStream(dst, src, errs)
	go copyStream(src, dst, errs)

	firstErr := <-errs
	if firstErr != nil {
		closeReadWriter(dst)
		closeReadWriter(src)
	}
	secondErr := <-errs

	log.Tracef("transport stream is finished")
	return errors.Join(normalizeTransportError(firstErr), normalizeTransportError(secondErr))
}

func copyStream(dst io.Writer, src io.Reader, errs chan<- error) {
	_, err := copyData(dst, src)
	if closeWriter, ok := dst.(interface{ CloseWrite() error }); ok {
		if closeErr := closeWriter.CloseWrite(); err == nil {
			err = closeErr
		}
	} else {
		closeReadWriter(dst)
	}
	errs <- err
}

func copyData(dst io.Writer, src io.Reader) (int64, error) {
	if _, ok := src.(io.WriterTo); ok {
		return io.Copy(dst, src)
	}
	if _, ok := dst.(io.ReaderFrom); ok {
		return io.Copy(dst, src)
	}
	buf := bufferPool.Get().(*[BufferSize]byte)
	defer bufferPool.Put(buf)
	return io.CopyBuffer(dst, src, buf[:])
}

func closeReadWriter(value any) {
	if closer, ok := value.(io.Closer); ok {
		_ = closer.Close()
	}
}

func normalizeTransportError(err error) error {
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || errors.Is(err, io.ErrClosedPipe) {
		return nil
	}
	return err
}

// NewServer creates a new Server instance with the given pool and configuration
func NewServer(pool *Pool, config ServerConfig) (*Server, error) {
	if pool == nil {
		return nil, errors.New("backend pool is required")
	}
	server := &Server{
		Pool:   pool,
		Config: &config,
	}
	if err := server.configureConnections(); err != nil {
		return nil, err
	}
	return server, nil
}
