package httpapi

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestLoggingCapturesRequestAndResponseFields(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	handler := withRequestLogging(logger, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	}))

	request := httptest.NewRequest(http.MethodPost, "/vocab?token=secret", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	handler.ServeHTTP(httptest.NewRecorder(), request)

	output := logs.String()
	for _, expected := range []string{
		"msg=request",
		"method=POST",
		"path=/vocab",
		"status=201",
		"duration_ms=",
		"bytes=7",
		"remote_addr=127.0.0.1:12345",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected log to contain %q, got %q", expected, output)
		}
	}
	if strings.Contains(output, "token=secret") {
		t.Fatalf("expected log to omit query string, got %q", output)
	}
}

func TestRequestLoggingSkipsHealthCheck(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	handler := withRequestLogging(logger, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	handler.ServeHTTP(httptest.NewRecorder(), request)

	if logs.Len() != 0 {
		t.Fatalf("expected health check to skip logging, got %q", logs.String())
	}
}
