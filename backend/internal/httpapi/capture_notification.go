package httpapi

import (
	"net/http"

	"vocabreview/backend/internal/service"
)

func (s *Server) handleCapture(w http.ResponseWriter, r *http.Request) {
	var req service.CaptureInput
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := s.app.CreateCapture(userIDFromContext(r.Context()), req)
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
	device, err := s.app.RegisterDevice(userIDFromContext(r.Context()), req.Platform, req.Token)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"device": device})
}

func (s *Server) handleNotificationJobs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.app.ListNotificationJobs(userIDFromContext(r.Context()))})
}
