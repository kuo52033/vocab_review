package httpapi

import (
	"net/http"
	"strings"

	"vocabreview/backend/internal/service"
)

func (s *Server) handleListVocab(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.ListVocab(userIDFromContext(r.Context()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
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
