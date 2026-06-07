package httpapi

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"vocabreview/backend/internal/clock"
	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/service"
)

type notificationJobsHTTPRepository struct {
	authenticatedHTTPRepository
	listErr error
}

func (r notificationJobsHTTPRepository) ListNotificationJobs(context.Context, string) ([]domain.NotificationJob, error) {
	return nil, r.listErr
}

func TestHandleNotificationJobsReturnsServerErrorOnRepositoryFailure(t *testing.T) {
	repo := notificationJobsHTTPRepository{listErr: errors.New("list notification jobs failed")}
	handler := NewServer(service.NewApp(repo, clock.RealClock{}), testLogger()).Handler()
	request := authenticatedRequest(http.MethodGet, "/notifications/jobs", nil)
	response := performRequest(handler, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d want %d body %s", response.Code, http.StatusInternalServerError, response.Body.String())
	}
}
