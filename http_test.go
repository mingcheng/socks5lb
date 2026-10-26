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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func EngineInstance(t *testing.T) *gin.Engine {
	t.Helper()
	server, err := NewServer(NewPool([]Backend{
		{
			Addr: "127.0.0.1:8888",
			CheckConfig: BackendCheckConfig{
				CheckURL: "https://www.taobao.com/robots.txt",
			},
		},
	}), ServerConfig{})

	require.NoError(t, err)
	require.NotNil(t, server)

	require.NoError(t, server.setupRouter())
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
	assert.Equal(t, "2 backend(s) added", w.Body.String())

	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodDelete, "/api/delete?addr=192.168.100.254:1086", nil)
	engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodDelete, "/api/delete?addr=<not>", nil)
	engine.ServeHTTP(w, req)
	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestHTTPServerIsolation(t *testing.T) {
	first := EngineInstance(t)
	second := EngineInstance(t)
	require.NotSame(t, first, second)

	w := httptest.NewRecorder()
	first.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/delete?addr=127.0.0.1:8888", nil))
	require.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	second.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/all", nil))
	var backends []Backend
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &backends))
	require.Len(t, backends, 1)
}

func TestHTTPGinModeMatchesDebug(t *testing.T) {
	mode := gin.ReleaseMode
	if DebugMode {
		mode = gin.DebugMode
	}
	require.Equal(t, mode, gin.Mode())
}

func TestHTTPAddInitialAlive(t *testing.T) {
	engine := EngineInstance(t)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/api/add",
		strings.NewReader(`[{"addr":"127.0.0.1:1080","check_config":{"initial_alive":true}}]`)))
	require.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/all?healthy=true", nil))
	var backends []Backend
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &backends))
	require.Len(t, backends, 1)
	require.Equal(t, "127.0.0.1:1080", backends[0].Addr)
}

func TestHTTPAddBatchIsAtomic(t *testing.T) {
	for _, duplicate := range []string{"127.0.0.1:8888", "127.0.0.1:1080"} {
		t.Run(duplicate, func(t *testing.T) {
			engine := EngineInstance(t)
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/api/add",
				strings.NewReader(`[{"addr":"127.0.0.1:1080"},{"addr":"`+duplicate+`"}]`)))
			require.Equal(t, http.StatusServiceUnavailable, w.Code)

			w = httptest.NewRecorder()
			engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/all", nil))
			var backends []Backend
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &backends))
			require.Len(t, backends, 1, "a rejected batch must not partially modify the pool")
			require.Equal(t, "127.0.0.1:8888", backends[0].Addr)
		})
	}
}

func TestHTTPInvalidInput(t *testing.T) {
	tests := []struct {
		name, method, path, body string
	}{
		{"invalid JSON", http.MethodPut, "/api/add", "["},
		{"missing address", http.MethodPut, "/api/add", `[{}]`},
		{"blank address", http.MethodPut, "/api/add", `[{"addr":" "}]`},
		{"empty batch", http.MethodPut, "/api/add", `[]`},
		{"null batch", http.MethodPut, "/api/add", `null`},
		{"missing delete address", http.MethodDelete, "/api/delete", ""},
		{"invalid health filter", http.MethodGet, "/api/all?healthy=invalid", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := EngineInstance(t)
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body)))
			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		})
	}
}

func TestHTTPConcurrentBackendUpdates(t *testing.T) {
	engine := EngineInstance(t)
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/api/add",
				strings.NewReader(`[{"addr":"127.0.0.1:1080"}]`)))
			assert.Contains(t, []int{http.StatusOK, http.StatusServiceUnavailable}, w.Code)
		})
	}
	wg.Wait()
}
