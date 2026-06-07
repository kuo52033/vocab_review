package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/repository"
)

const (
	testSessionToken = "sess_test"
	testUserID       = "usr_test"
	testUserEmail    = "test@example.com"
)

type authenticatedHTTPRepository struct {
	repository.AppRepository
}

func (r authenticatedHTTPRepository) HealthCheck(context.Context) error {
	return nil
}

func (r authenticatedHTTPRepository) GetSessionUser(_ context.Context, token string) (domain.Session, domain.User, bool, error) {
	session, user, ok := testSessionUser(token)
	return session, user, ok, nil
}

func testSessionUser(token string) (domain.Session, domain.User, bool) {
	if token == "" {
		return domain.Session{}, domain.User{}, false
	}
	return domain.Session{
			TokenHash: token,
			UserID:    testUserID,
			ExpiresAt: time.Now().Add(time.Hour),
		}, domain.User{
			ID:    testUserID,
			Email: testUserEmail,
		}, true
}

func authenticatedRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	req.Header.Set("Authorization", "Bearer "+testSessionToken)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func performRequest(handler http.Handler, request *http.Request) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
