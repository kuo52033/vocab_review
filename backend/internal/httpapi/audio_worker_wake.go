package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const audioWorkerWakeTokenHeader = "X-Audio-Worker-Wake-Token"

type AudioWorkerWake interface {
	Wake(ctx context.Context) error
}

type HTTPAudioWorkerWake struct {
	url    string
	token  string
	client *http.Client
}

func NewHTTPAudioWorkerWake(url, token string, client *http.Client) (*HTTPAudioWorkerWake, error) {
	url = strings.TrimSpace(url)
	token = strings.TrimSpace(token)
	if url == "" {
		return nil, nil
	}
	if token == "" {
		return nil, errors.New("audio worker wake token is required when wake URL is configured")
	}
	if client == nil {
		client = &http.Client{Timeout: time.Second}
	}
	return &HTTPAudioWorkerWake{url: url, token: token, client: client}, nil
}

func (w *HTTPAudioWorkerWake) Wake(ctx context.Context) error {
	if w == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, nil)
	if err != nil {
		return err
	}
	req.Header.Set(audioWorkerWakeTokenHeader, w.token)
	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("audio worker wake returned status %d", resp.StatusCode)
	}
	return nil
}

func (s *Server) wakeAudioWorker(ctx context.Context, audioJobEnqueued bool) {
	if !audioJobEnqueued || s.audioWorkerWake == nil {
		return
	}
	if err := s.audioWorkerWake.Wake(ctx); err != nil {
		s.logger.Warn("audio worker wake failed", "error", err)
	}
}
