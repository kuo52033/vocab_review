package httpapi

import (
	"context"
	"encoding/json"
	"io"
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

type reviewHTTPRepository struct {
	repository.AppRepository
	seenPagination repository.Pagination
}

func (r *reviewHTTPRepository) HealthCheck(context.Context) error { return nil }

func (r *reviewHTTPRepository) GetSessionUser(_ context.Context, token string) (domain.Session, domain.User, bool, error) {
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

func (r *reviewHTTPRepository) ListReviewHistory(_ context.Context, userID string, pagination repository.Pagination) ([]repository.ReviewHistoryEntry, int, error) {
	if userID != "usr_test" {
		return nil, 0, nil
	}
	r.seenPagination = pagination
	return []repository.ReviewHistoryEntry{}, 42, nil
}

func TestHandleReviewHistoryAcceptsPaginatedGetRequest(t *testing.T) {
	repo := &reviewHTTPRepository{}
	handler := NewServer(service.NewApp(repo, clock.RealClock{}), slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()
	request := httptest.NewRequest(http.MethodGet, "/reviews/history?limit=21&offset=0", nil)
	request.Header.Set("Authorization", "Bearer sess_test")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status: got %d want %d body %s", response.Code, http.StatusOK, response.Body.String())
	}
	if repo.seenPagination.Limit != 21 || repo.seenPagination.Offset != 0 {
		t.Fatalf("pagination: got %+v want limit 21 offset 0", repo.seenPagination)
	}
	var body service.ReviewHistoryPage
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Total != 42 || body.Limit != 21 || body.Offset != 0 {
		t.Fatalf("response page metadata: got total=%d limit=%d offset=%d", body.Total, body.Limit, body.Offset)
	}
}
