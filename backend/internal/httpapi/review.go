package httpapi

import (
	"net/http"

	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/service"
)

func (s *Server) registerReviewRoutes() {
	s.handleAuthenticated("GET /reviews/due", s.handleDueCards)
	s.handleAuthenticated("GET /reviews/history", s.handleReviewHistory)
	s.handleAuthenticated("GET /reviews/stats", s.handleReviewStats)
	s.handleAuthenticated("POST /reviews/{id}/grade", s.handleGradeReview)
}

func (s *Server) handleDueCards(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.DueCards(r.Context(), userIDFromContext(r.Context()))
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
	result, err := s.app.ReviewHistory(r.Context(), userIDFromContext(r.Context()), service.PageInput{Limit: page.Limit, Offset: page.Offset})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleReviewStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.app.ReviewStats(r.Context(), userIDFromContext(r.Context()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"stats": stats})
}

func (s *Server) handleGradeReview(w http.ResponseWriter, r *http.Request) {
	vocabID := r.PathValue("id")

	var req struct {
		Grade domain.ReviewGrade `json:"grade"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	state, err := s.app.GradeReview(r.Context(), userIDFromContext(r.Context()), vocabID, req.Grade)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"state": state})
}
