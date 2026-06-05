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

func (s *Server) handle(pattern string, handler http.HandlerFunc) {
	s.mux.HandleFunc(pattern, handler)
}

func (s *Server) handleWithAuthenticaed(pattern string, handler http.HandlerFunc) {
	s.mux.Handle(pattern, s.requireAuth(handler))
}

func (s *Server) routes() {
	s.handle("GET /healthz", s.handleHealth)
	s.registerAuthRoutes()
	s.registerVocabRoutes()
	s.registerReviewRoutes()
	s.registerCaptureRoutes()
}
