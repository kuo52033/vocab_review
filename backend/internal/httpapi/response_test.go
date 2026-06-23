package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthDoesNotRequireApp(t *testing.T) {
	handler := NewServer(nil, testLogger()).Handler()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := performRequest(handler, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status: got %d want %d", response.Code, http.StatusOK)
	}

	var payload map[string]string
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["status"] != "ok" {
		t.Fatalf("status payload: got %q want %q", payload["status"], "ok")
	}
}
