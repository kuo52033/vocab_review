package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"vocabreview/backend/internal/clock"
	"vocabreview/backend/internal/repository"
	"vocabreview/backend/internal/service"
)

type reviewHTTPRepository struct {
	authenticatedHTTPRepository
	seenPagination repository.Pagination
}

func (r *reviewHTTPRepository) ListReviewHistory(_ context.Context, userID string, pagination repository.Pagination) ([]repository.ReviewHistoryEntry, int, bool, error) {
	if userID != "usr_test" {
		return nil, 0, false, nil
	}
	r.seenPagination = pagination
	return []repository.ReviewHistoryEntry{}, 42, true, nil
}

func TestHandleReviewHistoryAcceptsPaginatedGetRequest(t *testing.T) {
	repo := &reviewHTTPRepository{}
	handler := NewServer(service.NewApp(repo, clock.RealClock{}), testLogger()).Handler()
	request := authenticatedRequest(http.MethodGet, "/reviews/history?limit=21&offset=0", nil)
	response := performRequest(handler, request)

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
	if body.Total != 42 || body.Limit != 21 || body.Offset != 0 || !body.HasNext {
		t.Fatalf("response page metadata: got total=%d limit=%d offset=%d hasNext=%v", body.Total, body.Limit, body.Offset, body.HasNext)
	}
}
