package httpapi

import (
	"net/http"
	"strings"

	"vocabreview/backend/internal/domain"
)

func (s *Server) handleDueCards(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.DueCards(userIDFromContext(r.Context()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleReviewHistory(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ReviewHistory(userIDFromContext(r.Context()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleGradeReview(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/reviews/")
	if !strings.HasSuffix(path, "/grade") {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	vocabID := strings.TrimSuffix(path, "/grade")
	var req struct {
		Grade domain.ReviewGrade `json:"grade"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	state, err := s.app.GradeReview(userIDFromContext(r.Context()), vocabID, req.Grade)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"state": state})
}
