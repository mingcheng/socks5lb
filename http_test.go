/*!*
 * Copyright (c) 2025 Hangzhou Guanwaii Technology Co., Ltd.
 *
 * This source code is licensed under the MIT License,
 * which is located in the LICENSE file in the source tree's root directory.
 *
 * File: http_test.go
 * Author: mingcheng (mingcheng@apache.org)
 * File Created: 2025-10-07 11:08:41
 *
 * Modified By: mingcheng (mingcheng@apache.org)
 * Last Modified: 2025-10-07 11:23:19
 */

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
