package httpapi

import (
	"net/http"
	"strings"

	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/service"
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
	page, err := parsePageQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := s.app.ReviewHistory(userIDFromContext(r.Context()), service.PageInput{Limit: page.Limit, Offset: page.Offset})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleReviewStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.app.ReviewStats(userIDFromContext(r.Context()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"stats": stats})
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
