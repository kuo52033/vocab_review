package httpapi

import (
	"errors"
	"net/http"

	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/service"
	"vocabreview/backend/internal/service/enrichment"
)

func (s *Server) registerVocabRoutes() {
	s.handleAuthenticated("GET /vocab", s.handleListVocab)
	s.handleAuthenticated("POST /vocab/autocomplete", s.handleAutocompleteVocab)
	s.handleAuthenticated("POST /vocab/bulk", s.handleBulkCreateVocab)
	s.handleAuthenticated("POST /vocab", s.handleCreateVocab)
	s.handleAuthenticated("GET /vocab/{id}/audio-url", s.handleVocabAudioURL)
	s.handleAuthenticated("PATCH /vocab/{id}", s.handleUpdateVocab)
	s.handleAuthenticated("DELETE /vocab/{id}", s.handleDeleteVocab)
}

func (s *Server) handleListVocab(w http.ResponseWriter, r *http.Request) {
	page, err := parsePageQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := s.app.ListVocab(r.Context(), userIDFromContext(r.Context()), service.ListVocabInput{
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
	result, err := s.app.CreateVocab(r.Context(), userIDFromContext(r.Context()), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.wakeAudioWorker(r.Context(), result.AudioJobEnqueued)
	status := http.StatusCreated
	if !result.Created {
		status = http.StatusOK
	}
	writeJSON(w, status, result)
}

func (s *Server) handleBulkCreateVocab(w http.ResponseWriter, r *http.Request) {
	var req service.BulkCreateVocabInput
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := s.app.BulkCreateVocab(r.Context(), userIDFromContext(r.Context()), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.wakeAudioWorker(r.Context(), result.AudioJobEnqueued)
	status := http.StatusCreated
	if result.CreatedCount == 0 {
		status = http.StatusOK
	}
	writeJSON(w, status, result)
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
	id := r.PathValue("id")

	var req service.CreateVocabInput
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := s.app.UpdateVocab(r.Context(), userIDFromContext(r.Context()), id, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.wakeAudioWorker(r.Context(), result.AudioJobEnqueued)
	writeJSON(w, http.StatusOK, map[string]any{"item": result.Item})
}

func (s *Server) handleVocabAudioURL(w http.ResponseWriter, r *http.Request) {
	vocabID := r.PathValue("id")
	if vocabID == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	url, err := s.app.VocabAudioURL(r.Context(), userIDFromContext(r.Context()), vocabID)
	if err != nil {
		writeError(w, vocabAudioURLStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": url})
}

func vocabAudioURLStatus(err error) int {
	switch {
	case errors.Is(err, service.ErrVocabAudioNotFound):
		return http.StatusNotFound
	case errors.Is(err, service.ErrVocabAudioNotReady):
		return http.StatusConflict
	case errors.Is(err, service.ErrVocabAudioURLUnavailable):
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

func (s *Server) handleDeleteVocab(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	item, err := s.app.ArchiveVocab(r.Context(), userIDFromContext(r.Context()), id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}
