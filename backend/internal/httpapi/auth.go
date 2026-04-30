package httpapi

import (
	"net/http"
)

func (s *Server) handleMagicLink(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email   string `json:"email"`
		BaseURL string `json:"base_url"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.BaseURL == "" {
		req.BaseURL = "http://localhost:8080"
	}
	result, err := s.app.RequestMagicLink(req.Email, req.BaseURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := s.app.VerifyMagicLink(req.Token)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}
