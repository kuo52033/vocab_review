package httpapi

import (
	"log/slog"
	"net/http"

	"vocabreview/backend/internal/service"
)

type Server struct {
	app    *service.App
	logger *slog.Logger
	mux    *http.ServeMux
}

func NewServer(app *service.App, logger *slog.Logger) *Server {
	server := &Server{
		app:    app,
		logger: logger,
		mux:    http.NewServeMux(),
	}
	server.routes()
	return server
}

func (s *Server) Handler() http.Handler {
	return withRequestLogging(s.logger, withCORS(s.mux))
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealth)

	s.mux.HandleFunc("POST /auth/magic-link", s.handleMagicLink)
	s.mux.HandleFunc("POST /auth/verify", s.handleVerify)

	s.mux.Handle("GET /vocab", s.requireAuth(http.HandlerFunc(s.handleListVocab)))
	s.mux.Handle("POST /vocab", s.requireAuth(http.HandlerFunc(s.handleCreateVocab)))
	s.mux.Handle("PATCH /vocab/", s.requireAuth(http.HandlerFunc(s.handleUpdateVocab)))
	s.mux.Handle("DELETE /vocab/", s.requireAuth(http.HandlerFunc(s.handleDeleteVocab)))

	s.mux.Handle("GET /reviews/due", s.requireAuth(http.HandlerFunc(s.handleDueCards)))
	s.mux.Handle("GET /reviews/history", s.requireAuth(http.HandlerFunc(s.handleReviewHistory)))
	s.mux.Handle("POST /reviews/", s.requireAuth(http.HandlerFunc(s.handleGradeReview)))

	s.mux.Handle("POST /captures", s.requireAuth(http.HandlerFunc(s.handleCapture)))
	s.mux.Handle("POST /devices/apns-token", s.requireAuth(http.HandlerFunc(s.handleDeviceToken)))
	s.mux.Handle("GET /notifications/jobs", s.requireAuth(http.HandlerFunc(s.handleNotificationJobs)))
}
