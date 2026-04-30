package repository

import (
	"slices"

	"vocabreview/backend/internal/domain"
)

func (s *Store) PutCapture(capture domain.CaptureSource) error {
	s.mu.Lock()
	s.CaptureSources[capture.ID] = capture
	s.mu.Unlock()
	return s.Save()
}

func (s *Store) UpsertDeviceToken(token domain.DeviceToken) error {
	s.mu.Lock()
	for id, current := range s.DeviceTokens {
		if current.UserID == token.UserID && current.Token == token.Token {
			token.ID = id
		}
	}
	s.DeviceTokens[token.ID] = token
	s.mu.Unlock()
	return s.Save()
}

func (s *Store) ListDeviceTokens(userID string) []domain.DeviceToken {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]domain.DeviceToken, 0)
	for _, token := range s.DeviceTokens {
		if token.UserID == userID {
			result = append(result, token)
		}
	}
	return result
}

func (s *Store) PutNotificationJob(job domain.NotificationJob) error {
	s.mu.Lock()
	s.NotificationJobs[job.ID] = job
	s.mu.Unlock()
	return s.Save()
}

func (s *Store) FindPendingJob(userID, vocabID string) (domain.NotificationJob, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, job := range s.NotificationJobs {
		if job.UserID == userID && job.VocabItemID == vocabID && job.Status == "pending" {
			return job, true
		}
	}
	return domain.NotificationJob{}, false
}

func (s *Store) ListNotificationJobs(userID string) []domain.NotificationJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]domain.NotificationJob, 0)
	for _, job := range s.NotificationJobs {
		if job.UserID == userID {
			result = append(result, job)
		}
	}
	slices.SortFunc(result, func(a, b domain.NotificationJob) int {
		if a.ScheduledAt.Before(b.ScheduledAt) {
			return -1
		}
		if a.ScheduledAt.After(b.ScheduledAt) {
			return 1
		}
		return 0
	})
	return result
}
