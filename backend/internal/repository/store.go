package repository

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"vocabreview/backend/internal/domain"
)

type Store struct {
	mu sync.RWMutex

	path string

	Users            map[string]domain.User            `json:"users"`
	Sessions         map[string]domain.Session         `json:"sessions"`
	MagicLinks       map[string]domain.MagicLinkToken  `json:"magic_links"`
	VocabItems       map[string]domain.VocabItem       `json:"vocab_items"`
	ReviewStates     map[string]domain.ReviewState     `json:"review_states"`
	ReviewLogs       map[string]domain.ReviewLog       `json:"review_logs"`
	CaptureSources   map[string]domain.CaptureSource   `json:"capture_sources"`
	DeviceTokens     map[string]domain.DeviceToken     `json:"device_tokens"`
	NotificationJobs map[string]domain.NotificationJob `json:"notification_jobs"`
}

func NewStore(path string) (*Store, error) {
	store := &Store{
		path:             path,
		Users:            map[string]domain.User{},
		Sessions:         map[string]domain.Session{},
		MagicLinks:       map[string]domain.MagicLinkToken{},
		VocabItems:       map[string]domain.VocabItem{},
		ReviewStates:     map[string]domain.ReviewState{},
		ReviewLogs:       map[string]domain.ReviewLog{},
		CaptureSources:   map[string]domain.CaptureSource{},
		DeviceTokens:     map[string]domain.DeviceToken{},
		NotificationJobs: map[string]domain.NotificationJob{},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store, nil
		}
		return nil, err
	}

	if len(data) == 0 {
		return store, nil
	}

	if err := json.Unmarshal(data, store); err != nil {
		return nil, err
	}

	store.path = path
	return store, nil
}

func (s *Store) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0o644)
}
