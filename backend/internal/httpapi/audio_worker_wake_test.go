package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPAudioWorkerWakePostsToken(t *testing.T) {
	var sawToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method: got %s want %s", r.Method, http.MethodPost)
		}
		sawToken = r.Header.Get(audioWorkerWakeTokenHeader)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	wake, err := NewHTTPAudioWorkerWake(server.URL, "secret", server.Client())
	if err != nil {
		t.Fatalf("new wake: %v", err)
	}
	if err := wake.Wake(context.Background()); err != nil {
		t.Fatalf("wake: %v", err)
	}
	if sawToken != "secret" {
		t.Fatalf("token: got %q want secret", sawToken)
	}
}

func TestHTTPAudioWorkerWakeRequiresTokenWithURL(t *testing.T) {
	if _, err := NewHTTPAudioWorkerWake("http://example.test/wake", "", nil); err == nil {
		t.Fatal("expected error")
	}
}
