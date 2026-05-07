package postgres

import (
	"context"

	"vocabreview/backend/internal/domain"
)

func (s *Store) ListNotificationJobs(ctx context.Context, userID string) ([]domain.NotificationJob, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, vocab_item_id, scheduled_at, sent_at, status, message
		FROM notification_jobs
		WHERE user_id = $1
		ORDER BY scheduled_at ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := make([]domain.NotificationJob, 0)
	for rows.Next() {
		var job domain.NotificationJob
		if err := rows.Scan(&job.ID, &job.UserID, &job.VocabItemID, &job.ScheduledAt, &job.SentAt, &job.Status, &job.Message); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}
