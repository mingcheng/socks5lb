package socks5lb

import (
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func EngineInstance(t *testing.T) *gin.Engine {
	server, err := NewServer(NewPool([]Backend{
		{
			Addr: "127.0.0.1:8888",
			CheckConfig: BackendCheckConfig{
				CheckURL: "https://www.taobao.com/robots.txt",
			},
		},
	}), ServerConfig{})

	assert.NoError(t, err)
	assert.NotNil(t, server)

	_ = server.setupRouter()
	return server.Engine()
}

func TestServer_HTTPVersion(t *testing.T) {
	engine := EngineInstance(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/version", nil)

	engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestServer_HTTPAll(t *testing.T) {
	engine := EngineInstance(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/all", nil)

	engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestServer_HTTPPutAndDelete(t *testing.T) {
	engine := EngineInstance(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/api/add", strings.NewReader(`[
  {
    "addr": "192.168.100.254:1086",
    "check_config": {
      "check_url": "https://www.taobao.com/robots.txt"
    }
  },
  {
    "addr": "192.168.111.254:1086",
    "check_config": {
      "initial_alive": true
    }
  }
	]`))

	engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, w.Body.String(), "2")

	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodDelete, "/api/delete?addr=192.168.100.254:1086", nil)
	engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodDelete, "/api/delete?addr=<not>", nil)
	engine.ServeHTTP(w, req)
	assert.NotEqual(t, http.StatusOK, w.Code)
}
