package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"vocabreview/backend/internal/clock"
	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/service"
	"vocabreview/backend/internal/service/enrichment"
)

type autocompleteHTTPRepository struct {
	authenticatedHTTPRepository
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
	request := authenticatedRequest(http.MethodPost, "/vocab/autocomplete", bytes.NewBufferString(`{"items":[{"term":"serendipity"}]}`))
	response := performRequest(handler, request)

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
	request := authenticatedRequest(http.MethodPost, "/vocab/autocomplete", bytes.NewBufferString(`{"items":[]}`))
	response := performRequest(handler, request)

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
	return NewServer(app, testLogger()).Handler()
}
