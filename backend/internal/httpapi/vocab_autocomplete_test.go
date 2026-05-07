package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"vocabreview/backend/internal/clock"
	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/repository"
	"vocabreview/backend/internal/service"
	"vocabreview/backend/internal/service/enrichment"
)

type autocompleteHTTPRepository struct {
	repository.AppRepository
}

func (r autocompleteHTTPRepository) HealthCheck(context.Context) error { return nil }

func (r autocompleteHTTPRepository) GetSessionUser(_ context.Context, token string) (domain.Session, domain.User, bool, error) {
	if token != "sess_test" {
		return domain.Session{}, domain.User{}, false, nil
	}
	return domain.Session{
			Token:     token,
			UserID:    "usr_test",
			ExpiresAt: time.Now().Add(time.Hour),
		}, domain.User{
			ID:    "usr_test",
			Email: "test@example.com",
		}, true, nil
}

type autocompleteHTTPEnricher struct {
	suggestions []enrichment.Suggestion
	err         error
}

func (e autocompleteHTTPEnricher) Autocomplete(context.Context, []enrichment.Item) ([]enrichment.Suggestion, error) {
	return e.suggestions, e.err
}

func TestHandleAutocompleteVocabReturnsSuggestions(t *testing.T) {
	handler := newAutocompleteHandler(t, autocompleteHTTPEnricher{
		suggestions: []enrichment.Suggestion{{
			Term:            "serendipity",
			Meaning:         "a fortunate discovery",
			ExampleSentence: "Finding the cafe was pure serendipity.",
			PartOfSpeech:    domain.PartOfSpeechNoun,
		}},
	})
	response := performAutocompleteRequest(handler, `{"items":[{"term":"serendipity"}]}`)

	if response.Code != http.StatusOK {
		t.Fatalf("status: got %d want %d body %s", response.Code, http.StatusOK, response.Body.String())
	}
	var body struct {
		Items []enrichment.Suggestion `json:"items"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Items) != 1 || body.Items[0].Meaning != "a fortunate discovery" {
		t.Fatalf("unexpected response: %#v", body.Items)
	}
}

func TestHandleAutocompleteVocabMapsValidationErrors(t *testing.T) {
	handler := newAutocompleteHandler(t, autocompleteHTTPEnricher{err: enrichment.ErrEmptyBatch})
	response := performAutocompleteRequest(handler, `{"items":[]}`)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d want %d body %s", response.Code, http.StatusBadRequest, response.Body.String())
	}
	var body map[string]string
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] != enrichment.ErrEmptyBatch.Error() {
		t.Fatalf("error: got %q want %q", body["error"], enrichment.ErrEmptyBatch.Error())
	}
}

func newAutocompleteHandler(t *testing.T, enricher service.VocabEnricher) http.Handler {
	t.Helper()
	app := service.NewAppWithEnricher(autocompleteHTTPRepository{}, clock.RealClock{}, enricher)
	return NewServer(app, slog.New(slog.NewTextHandler(ioDiscard{}, nil))).Handler()
}

func performAutocompleteRequest(handler http.Handler, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/vocab/autocomplete", bytes.NewBufferString(body))
	request.Header.Set("Authorization", "Bearer sess_test")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) {
	return len(p), nil
}
