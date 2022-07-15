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

func (s *Server) setupAPIRouter(apiGroup *gin.RouterGroup) (err error) {
	apiGroup.GET("all", func(c *gin.Context) {
		backends := s.Pool.All()

		printHealthy, _ := strconv.ParseBool(c.Query("healthy"))
		if printHealthy {
			backends = s.Pool.AllHealthy()
		}

		c.JSON(http.StatusOK, backends)
	})

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

	apiGroup.PUT("add", func(c *gin.Context) {
		var backends []Backend

		if err := c.ShouldBindJSON(&backends); err != nil {
			c.String(http.StatusNoContent, err.Error())
			return
		}

		for _, backend := range backends {
			//if backend.CheckConfig.CheckURL == "" {
			//	backend.CheckConfig.CheckURL = "https://www.taobao.com"
			//}
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

func (s *Server) setupRouter() (err error) {
	if engine != nil {
		return fmt.Errorf("the Gin engine is alreay instanced, maybe is running")
	}

	engine = gin.New()
	engine.Use(ginlogrus.Logger(log.New(), "http", false, true, os.Stdout, log.TraceLevel))
	engine.Use(gin.Recovery())

	err = s.setupAPIRouter(engine.Group("/api"))

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

func (s *Server) Engine() *gin.Engine {
	return engine
}
