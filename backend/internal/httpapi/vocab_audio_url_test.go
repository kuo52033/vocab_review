package httpapi

import (
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
)

type vocabAudioURLRepository struct {
	repository.AppRepository
	item domain.VocabItem
}

func (r vocabAudioURLRepository) HealthCheck(context.Context) error { return nil }

func (r vocabAudioURLRepository) GetSessionUser(_ context.Context, token string) (domain.Session, domain.User, bool, error) {
	if token == "" {
		return domain.Session{}, domain.User{}, false, nil
	}
	return domain.Session{
			TokenHash: token,
			UserID:    "usr_test",
			ExpiresAt: time.Now().Add(time.Hour),
		}, domain.User{
			ID:    "usr_test",
			Email: "test@example.com",
		}, true, nil
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
	handler := NewServer(app, slog.New(slog.NewTextHandler(ioDiscard{}, nil))).Handler()
	request := httptest.NewRequest(http.MethodGet, "/vocab/voc_test/audio-url", nil)
	request.Header.Set("Authorization", "Bearer sess_test")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

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
