package httpapi

import (
	"log/slog"
	"net/http"

	"vocabreview/backend/internal/service"
)

type Server struct {
	app             *service.App
	logger          *slog.Logger
	mux             *http.ServeMux
	audioWorkerWake AudioWorkerWake
}

func NewServer(app *service.App, logger *slog.Logger) *Server {
	return NewServerWithAudioWorkerWake(app, logger, nil)
}

func NewServerWithAudioWorkerWake(app *service.App, logger *slog.Logger, audioWorkerWake AudioWorkerWake) *Server {
	server := &Server{
		app:             app,
		logger:          logger,
		mux:             http.NewServeMux(),
		audioWorkerWake: audioWorkerWake,
	}
	server.routes()
	return server
}

func (s *Server) Handler() http.Handler {
	return withRequestLogging(s.logger, withCORS(s.mux))
}

func (s *Server) handle(pattern string, handler http.HandlerFunc) {
	s.mux.HandleFunc(pattern, handler)
}

func (s *Server) handleAuthenticated(pattern string, handler http.HandlerFunc) {
	s.mux.Handle(pattern, s.requireAuth(handler))
}

func (s *Server) routes() {
	s.handle("GET /healthz", s.handleHealth)
	s.registerAuthRoutes()
	s.registerVocabRoutes()
	s.registerReviewRoutes()
	s.registerCaptureRoutes()
}
