package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/service"
	"vocabreview/backend/internal/service/enrichment"
)

func (s *Server) handleListVocab(w http.ResponseWriter, r *http.Request) {
	page, err := parsePageQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := s.app.ListVocab(userIDFromContext(r.Context()), service.ListVocabInput{
		Limit:  page.Limit,
		Offset: page.Offset,
		Query:  r.URL.Query().Get("q"),
		Status: domain.ReviewStatus(r.URL.Query().Get("status")),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleCreateVocab(w http.ResponseWriter, r *http.Request) {
	var req service.CreateVocabInput
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	item, state, err := s.app.CreateVocab(userIDFromContext(r.Context()), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"item": item, "state": state})
}

func (s *Server) handleAutocompleteVocab(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Items []enrichment.Item `json:"items"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	items, err := s.app.AutocompleteVocab(r.Context(), req.Items)
	if err != nil {
		writeError(w, autocompleteVocabStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func autocompleteVocabStatus(err error) int {
	switch {
	case errors.Is(err, service.ErrEnrichmentNotConfigured):
		return http.StatusBadGateway
	case errors.Is(err, enrichment.ErrEmptyBatch),
		errors.Is(err, enrichment.ErrBatchTooLarge),
		errors.Is(err, enrichment.ErrTermRequired):
		return http.StatusBadRequest
	case errors.Is(err, enrichment.ErrProviderFailed):
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

func (s *Server) handleUpdateVocab(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/vocab/")
	var req service.CreateVocabInput
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	item, err := s.app.UpdateVocab(userIDFromContext(r.Context()), id, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (s *Server) handleDeleteVocab(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/vocab/")
	item, err := s.app.ArchiveVocab(userIDFromContext(r.Context()), id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}
