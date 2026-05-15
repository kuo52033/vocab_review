package httpapi

import (
	"context"
	"errors"
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

type notificationJobsHTTPRepository struct {
	repository.AppRepository
	listErr error
}

func (r notificationJobsHTTPRepository) GetSessionUser(_ context.Context, token string) (domain.Session, domain.User, bool, error) {
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

func (r notificationJobsHTTPRepository) ListNotificationJobs(context.Context, string) ([]domain.NotificationJob, error) {
	return nil, r.listErr
}

func TestHandleNotificationJobsReturnsServerErrorOnRepositoryFailure(t *testing.T) {
	repo := notificationJobsHTTPRepository{listErr: errors.New("list notification jobs failed")}
	handler := NewServer(service.NewApp(repo, clock.RealClock{}), slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()
	request := httptest.NewRequest(http.MethodGet, "/notifications/jobs", nil)
	request.Header.Set("Authorization", "Bearer sess_test")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d want %d body %s", response.Code, http.StatusInternalServerError, response.Body.String())
	}
}
