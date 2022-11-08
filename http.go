/**
 * File: http.go
 * Author: Ming Cheng<mingcheng@outlook.com>
 *
 * Created Date: Saturday, July 9th 2022, 7:42:02 pm
 * Last Modified: Friday, July 15th 2022, 5:33:53 pm
 *
 * http://www.opensource.org/licenses/MIT
 */

package socks5lb

import (
	"fmt"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"strconv"
	"time"
)
import "github.com/rocksolidlabs/gin-logrus"

var engine *gin.Engine

func init() {
	if DebugMode {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	gin.DisableConsoleColor()
}

// setupAPIRouter to handle the APIRouter
func (s *Server) setupAPIRouter(apiGroup *gin.RouterGroup) (err error) {

	// to show all backends
	apiGroup.GET("all", func(c *gin.Context) {
		backends := s.Pool.All()

		// if shows healthy backends only
		printHealthy, _ := strconv.ParseBool(c.Query("healthy"))
		if printHealthy {
			backends = s.Pool.AllHealthy()
		}

		if len(backends) <= 0 {
			err = fmt.Errorf("the backends are empty, so return empty json")
		}

		c.JSON(http.StatusOK, backends)
	})

	// to delete a single backend
	apiGroup.DELETE("delete", func(c *gin.Context) {
		addr := c.Query("addr")
		if addr == "" {
			c.String(http.StatusBadRequest, "address is empty")
			return
		}
		log.Tracef("the be removed server address is %s", addr)

		err := s.Pool.Remove(addr)
		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}

		c.String(http.StatusOK, fmt.Sprintf("server %s is removed", addr))
	})

	// batch add backends
	apiGroup.PUT("add", func(c *gin.Context) {
		var backends []Backend

		if err := c.ShouldBindJSON(&backends); err != nil {
			c.String(http.StatusNoContent, err.Error())
			return
		}

		for _, backend := range backends {
			err = s.Pool.Add(&backend)
			if err != nil {
				c.String(http.StatusServiceUnavailable, err.Error())
				return
			}
		}

		c.String(http.StatusOK, fmt.Sprintf("%d", len(backends)))
	})

	return
}

// setupRouter to set up the http server routers
func (s *Server) setupRouter() (err error) {
	if engine != nil {
		return fmt.Errorf("the Gin engine is alreay instanced, maybe is running")
	}

	// gin default config
	engine = gin.New()
	engine.Use(ginlogrus.Logger(log.New(), "http", false, true, os.Stdout, log.TraceLevel))
	engine.Use(gin.Recovery())

	err = s.setupAPIRouter(engine.Group("/api"))

	// show basic information
	engine.GET("/version", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"name":         AppName,
			"version":      Version,
			"build_commit": BuildCommit,
			"build_date":   BuildDate,
			"uptime":       time.Now().Sub(StartTime),
		})
	})
	return
}

// ListenHTTPAdmin is not implemented by default
func (s *Server) ListenHTTPAdmin(addr string) (err error) {
	if err = s.setupRouter(); err != nil {
		return
	}

	return engine.Run(addr)
}

// Engine returns the main http engine for testing purposes
func (s *Server) Engine() *gin.Engine {
	return engine
}
