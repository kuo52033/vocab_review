package repository

import (
	"slices"
	"time"

	"vocabreview/backend/internal/domain"
)

func (s *Store) PutVocab(item domain.VocabItem, state domain.ReviewState) error {
	s.mu.Lock()
	s.VocabItems[item.ID] = item
	s.ReviewStates[item.ID] = state
	s.mu.Unlock()
	return s.Save()
}

func (s *Store) UpdateVocab(item domain.VocabItem) error {
	s.mu.Lock()
	s.VocabItems[item.ID] = item
	s.mu.Unlock()
	return s.Save()
}

func (s *Store) GetVocab(id string) (domain.VocabItem, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.VocabItems[id]
	return item, ok
}

func (s *Store) ListVocabByUser(userID string) []domain.VocabItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.VocabItem, 0)
	for _, item := range s.VocabItems {
		if item.UserID == userID {
			items = append(items, item)
		}
	}
	slices.SortFunc(items, func(a, b domain.VocabItem) int {
		if a.CreatedAt.Before(b.CreatedAt) {
			return -1
		}
		if a.CreatedAt.After(b.CreatedAt) {
			return 1
		}
		return 0
	})
	return items
}

func (s *Store) GetReviewState(vocabID string) (domain.ReviewState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, ok := s.ReviewStates[vocabID]
	return state, ok
}

func (s *Store) PutReviewResult(state domain.ReviewState, log domain.ReviewLog) error {
	s.mu.Lock()
	s.ReviewStates[state.VocabItemID] = state
	s.ReviewLogs[log.ID] = log
	s.mu.Unlock()
	return s.Save()
}

func (s *Store) ListDueStates(userID string, now time.Time) []domain.ReviewState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	due := make([]domain.ReviewState, 0)
	for _, state := range s.ReviewStates {
		if state.UserID == userID && !state.NextDueAt.After(now) {
			due = append(due, state)
		}
	}
	slices.SortFunc(due, func(a, b domain.ReviewState) int {
		if a.NextDueAt.Before(b.NextDueAt) {
			return -1
		}
		if a.NextDueAt.After(b.NextDueAt) {
			return 1
		}
		return 0
	})
	return due
}
