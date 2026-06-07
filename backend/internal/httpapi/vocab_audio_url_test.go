package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"vocabreview/backend/internal/clock"
	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/service"
)

type vocabAudioURLRepository struct {
	authenticatedHTTPRepository
	item domain.VocabItem
}

func (r vocabAudioURLRepository) GetVocab(_ context.Context, id string) (domain.VocabItem, bool, error) {
	if id != r.item.ID {
		return domain.VocabItem{}, false, nil
	}
	return r.item, true, nil
}

type vocabAudioURLSigner struct{}

func (vocabAudioURLSigner) SignVocabAudioURL(context.Context, string) (string, error) {
	return "https://signed.example.com/audio.mp3", nil
}

func TestHandleVocabAudioURLReturnsSignedURL(t *testing.T) {
	app := service.NewAppWithVocabAudioConfigAndSigner(
		vocabAudioURLRepository{item: domain.VocabItem{
			ID:     "voc_test",
			UserID: "usr_test",
			Term:   "serendipity",
			Audio: &domain.VocabAudio{
				Status:     "ready",
				StorageKey: "audio/openai/gpt-4o-mini-tts/alloy/hash.mp3",
			},
		}},
		clock.RealClock{},
		nil,
		service.AuthConfig{Environment: "development"},
		nil,
		service.VocabAudioConfig{Enabled: true},
		vocabAudioURLSigner{},
	)
	handler := NewServer(app, testLogger()).Handler()
	request := authenticatedRequest(http.MethodGet, "/vocab/voc_test/audio-url", nil)
	response := performRequest(handler, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status: got %d want %d body %s", response.Code, http.StatusOK, response.Body.String())
	}
	var body map[string]string
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["url"] != "https://signed.example.com/audio.mp3" {
		t.Fatalf("url: got %q", body["url"])
	}
}
