package enrichment

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"vocabreview/backend/internal/domain"
)

func TestOpenAIProviderCompletesBatch(t *testing.T) {
	var authHeader string
	var requestedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		requestedPath = r.URL.Path
		var req openAIChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != "test-model" {
			t.Fatalf("model: got %q want test-model", req.Model)
		}
		_ = json.NewEncoder(w).Encode(openAIChatResponse{Choices: []openAIChoice{{Message: openAIMessage{Content: `{"items":[{"term":"serendipity","meaning":"happy accident","example_sentence":"It was serendipity.","part_of_speech":"noun","error":""}]}`}}}})
	}))
	defer server.Close()

	provider := NewOpenAIProvider(server.URL, "secret", "test-model", server.Client())
	result, err := provider.Complete(context.Background(), []Item{{Term: "serendipity"}})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if requestedPath != "/chat/completions" {
		t.Fatalf("path: got %q want /chat/completions", requestedPath)
	}
	if authHeader != "Bearer secret" {
		t.Fatalf("auth header: got %q", authHeader)
	}
	if len(result) != 1 || result[0].Meaning != "happy accident" || result[0].PartOfSpeech != domain.PartOfSpeechNoun {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestOpenAIProviderRejectsInvalidJSONContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(openAIChatResponse{Choices: []openAIChoice{{Message: openAIMessage{Content: `not-json`}}}})
	}))
	defer server.Close()

	provider := NewOpenAIProvider(server.URL, "secret", "test-model", server.Client())
	if _, err := provider.Complete(context.Background(), []Item{{Term: "serendipity"}}); err == nil {
		t.Fatal("expected invalid JSON error")
	}
}
