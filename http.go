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
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	ginlogrus "github.com/rocksolidlabs/gin-logrus"
)

var engine *gin.Engine

func init() {
	if DebugMode {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	gin.DisableConsoleColor()
}

// setupAPIRouter configures the API routes for backend management
func (s *Server) setupAPIRouter(apiGroup *gin.RouterGroup) (err error) {

	// GET /api/all - List all backends, optionally filter by health status
	apiGroup.GET("all", func(c *gin.Context) {
		backends := s.Pool.All()

		// Filter to show only healthy backends if requested
		printHealthy, _ := strconv.ParseBool(c.Query("healthy"))
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
			c.String(http.StatusBadRequest, err.Error())
			return
		}

		c.String(http.StatusOK, fmt.Sprintf("backend %s removed successfully", addr))
	})

	// PUT /api/add - Add one or more backends to the pool
	apiGroup.PUT("add", func(c *gin.Context) {
		var backends []Backend

		if err := c.ShouldBindJSON(&backends); err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}

		// Add all backends, fail if any addition fails
		for _, backend := range backends {
			err = s.Pool.Add(&backend)
			if err != nil {
				c.String(http.StatusServiceUnavailable, err.Error())
				return
			}
		}

		c.String(http.StatusOK, fmt.Sprintf("%d backend(s) added", len(backends)))
	})

	return
}

// setupRouter configures the HTTP server routes and middleware
func (s *Server) setupRouter() (err error) {
	if engine != nil {
		return fmt.Errorf("gin engine is already initialized, server may be running")
	}

	// Initialize Gin with custom middleware
	engine = gin.New()
	engine.Use(ginlogrus.Logger(log.New(), "http", false, true, os.Stdout, log.TraceLevel))
	engine.Use(gin.Recovery())

	// Setup API routes under /api
	err = s.setupAPIRouter(engine.Group("/api"))

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
	return
}

// ListenHTTPAdmin starts the HTTP administration server
func (s *Server) ListenHTTPAdmin(addr string) (err error) {
	if err = s.setupRouter(); err != nil {
		return
	}

	log.Infof("starting HTTP admin interface on %s", addr)
	return engine.Run(addr)
}

// Engine returns the main http engine for testing purposes
func (s *Server) Engine() *gin.Engine {
	return engine
}
