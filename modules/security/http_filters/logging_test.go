package http_filters

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/global"
)

func TestLoggingFilterSkipsAccessLogWhenDisabled(t *testing.T) {
	oldEnv := global.Env()
	testEnv := env.EmptyEnv()
	testEnv.SystemConfig.PathConfig.Data = t.TempDir()
	testEnv.SystemConfig.PathConfig.Log = t.TempDir()
	testEnv.SystemConfig.WebAppConfig.AccessLog = false
	global.RegisterEnv(testEnv)
	defer global.RegisterEnv(oldEnv)

	accessLogHandler = nil
	defer func() { accessLogHandler = nil }()

	filter := &LoggingFilter{}
	handler := filter.ApplyFilter(http.MethodGet, "/hello", nil, func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	resp := httptest.NewRecorder()
	handler(resp, req, nil)

	accessLogPath := filepath.Join(testEnv.GetLogDir(), "access.log")
	if _, err := os.Stat(accessLogPath); !os.IsNotExist(err) {
		t.Fatalf("expected access log file to be absent when disabled, stat err=%v", err)
	}
}

func TestLoggingFilterWritesAccessLogWhenEnabled(t *testing.T) {
	oldEnv := global.Env()
	testEnv := env.EmptyEnv()
	testEnv.SystemConfig.PathConfig.Data = t.TempDir()
	testEnv.SystemConfig.PathConfig.Log = t.TempDir()
	testEnv.SystemConfig.WebAppConfig.AccessLog = true
	global.RegisterEnv(testEnv)
	defer global.RegisterEnv(oldEnv)

	accessLogHandler = nil
	defer func() { accessLogHandler = nil }()

	filter := &LoggingFilter{}
	handler := filter.ApplyFilter(http.MethodGet, "/hello", nil, func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	resp := httptest.NewRecorder()
	handler(resp, req, nil)

	accessLogPath := filepath.Join(testEnv.GetLogDir(), "access.log")
	if _, err := os.Stat(accessLogPath); err != nil {
		t.Fatalf("expected access log file to exist when enabled, got %v", err)
	}
}
