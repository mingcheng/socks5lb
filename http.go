/*!*
 * Copyright (c) 2025 Hangzhou Guanwaii Technology Co., Ltd.
 *
 * This source code is licensed under the MIT License,
 * which is located in the LICENSE file in the source tree's root directory.
 *
 * File: http.go
 * Author: mingcheng (mingcheng@apache.org)
 * File Created: Saturday, July 9th 2022, 7:42:02 pm
 *
 * Modified By: mingcheng (mingcheng@apache.org)
 * Last Modified: 2025-10-07 11:22:09
 */

package socks5lb

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	ginlogrus "github.com/rocksolidlabs/gin-logrus"
)

func init() {
	if DebugMode {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	gin.DisableConsoleColor()
}

// setupAPIRouter configures the API routes for backend management
func (s *Server) setupAPIRouter(apiGroup *gin.RouterGroup) {

	// GET /api/all - List all backends, optionally filter by health status
	apiGroup.GET("all", func(c *gin.Context) {
		backends := s.Pool.All()

		// Filter to show only healthy backends if requested
		var printHealthy bool
		if value := c.Query("healthy"); value != "" {
			var err error
			printHealthy, err = strconv.ParseBool(value)
			if err != nil {
				c.String(http.StatusBadRequest, "healthy must be a boolean")
				return
			}
		}
		if printHealthy {
			backends = s.Pool.AllHealthy()
		}

		c.JSON(http.StatusOK, backends)
	})

	// DELETE /api/delete - Remove a backend from the pool
	apiGroup.DELETE("delete", func(c *gin.Context) {
		addr := c.Query("addr")
		if addr == "" {
			c.String(http.StatusBadRequest, "address is empty")
			return
		}
		log.Tracef("removing backend with address: %s", addr)

		err := s.Pool.Remove(addr)
		if err != nil {
			c.String(http.StatusBadRequest, "%s", err.Error())
			return
		}

		c.String(http.StatusOK, "backend %s removed successfully", addr)
	})

	// PUT /api/add - Add one or more backends to the pool
	apiGroup.PUT("add", func(c *gin.Context) {
		var backends []Backend

		if err := c.ShouldBindJSON(&backends); err != nil {
			c.String(http.StatusBadRequest, "%s", err.Error())
			return
		}
		if len(backends) == 0 {
			c.String(http.StatusBadRequest, "at least one backend is required")
			return
		}
		candidates := make([]*Backend, 0, len(backends))
		for i := range backends {
			backend := &backends[i]
			if strings.TrimSpace(backend.Addr) == "" {
				c.String(http.StatusBadRequest, "backend address is empty")
				return
			}
			backend.SetAlive(backend.CheckConfig.InitialAlive)
			candidates = append(candidates, backend)
		}
		if err := s.Pool.addBackends(candidates); err != nil {
			c.String(http.StatusServiceUnavailable, "%s", err.Error())
			return
		}

		c.String(http.StatusOK, "%d backend(s) added", len(backends))
	})
}

// setupRouter configures the HTTP server routes and middleware
func (s *Server) setupRouter() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Pool == nil {
		return errors.New("backend pool is required")
	}
	if s.engine != nil {
		return nil
	}

	// Initialize Gin with custom middleware
	engine := gin.New()
	engine.Use(ginlogrus.Logger(log.New(), "http", false, true, os.Stdout, log.TraceLevel))
	engine.Use(gin.Recovery())

	// Setup API routes under /api
	s.setupAPIRouter(engine.Group("/api"))

	// GET /version - Show application version and status information
	engine.GET("/version", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"name":         AppName,
			"version":      Version,
			"build_commit": BuildCommit,
			"build_date":   BuildDate,
			"uptime":       time.Since(StartTime).String(),
		})
	})
	s.engine = engine
	return nil
}

// ListenHTTPAdmin starts the HTTP administration server
func (s *Server) ListenHTTPAdmin(addr string) error {
	server, listener, err := s.listenHTTPAdmin(addr)
	if err != nil {
		return err
	}
	return serveHTTPAdmin(server, listener)
}

func (s *Server) listenHTTPAdmin(addr string) (*http.Server, net.Listener, error) {
	if err := s.setupRouter(); err != nil {
		return nil, nil, err
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, fmt.Errorf("listen HTTP admin: %w", err)
	}
	server := &http.Server{
		Handler:           s.Engine(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		BaseContext: func(net.Listener) context.Context {
			return s.connectionContext()
		},
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		_ = listener.Close()
		return nil, nil, net.ErrClosed
	}
	if s.httpServer != nil {
		_ = listener.Close()
		return nil, nil, errors.New("HTTP server is already started")
	}
	s.httpServer = server
	s.httpListener = listener
	log.Infof("starting HTTP admin interface on %s", addr)
	return server, listener, nil
}

func serveHTTPAdmin(server *http.Server, listener net.Listener) error {
	defer listener.Close()
	err := server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

// Engine returns the main http engine for testing purposes
func (s *Server) Engine() *gin.Engine {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.engine
}
