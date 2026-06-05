package httpapi

import (
	"net/http"

	"vocabreview/backend/internal/service"
)

func (s *Server) registerCaptureRoutes() {
	s.handleWithAuthenticaed("POST /captures", s.handleCapture)
	s.handleWithAuthenticaed("POST /devices/apns-token", s.handleDeviceToken)
	s.handleWithAuthenticaed("GET /notifications/jobs", s.handleNotificationJobs)
}

func (s *Server) handleCapture(w http.ResponseWriter, r *http.Request) {
	var req service.CaptureInput
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := s.app.CreateCapture(r.Context(), userIDFromContext(r.Context()), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) handleDeviceToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Platform string `json:"platform"`
		Token    string `json:"token"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	device, err := s.app.RegisterDevice(r.Context(), userIDFromContext(r.Context()), req.Platform, req.Token)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"device": device})
}

func (s *Server) handleNotificationJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.app.ListNotificationJobs(r.Context(), userIDFromContext(r.Context()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": jobs})
}
