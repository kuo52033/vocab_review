package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

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

func (s *Store) ClaimDueNotificationJobs(ctx context.Context, now time.Time, limit int) ([]domain.NotificationJob, error) {
	if limit <= 0 {
		limit = 50
	}
	var jobs []domain.NotificationJob
	err := withTx(ctx, s.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			WITH claimed AS (
				SELECT id
				FROM notification_jobs
				WHERE status = 'pending'
				  AND scheduled_at <= $1
				ORDER BY scheduled_at ASC
				LIMIT $2
				FOR UPDATE SKIP LOCKED
			)
			UPDATE notification_jobs AS jobs
			SET status = 'processing'
			FROM claimed
			WHERE jobs.id = claimed.id
			RETURNING jobs.id, jobs.user_id, jobs.vocab_item_id, jobs.scheduled_at, jobs.sent_at, jobs.status, jobs.message
		`, now.UTC(), limit)
		if err != nil {
			return err
		}
		defer rows.Close()

		claimed := make([]domain.NotificationJob, 0)
		for rows.Next() {
			var job domain.NotificationJob
			if err := rows.Scan(&job.ID, &job.UserID, &job.VocabItemID, &job.ScheduledAt, &job.SentAt, &job.Status, &job.Message); err != nil {
				return err
			}
			claimed = append(claimed, job)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		jobs = claimed
		return nil
	})
	return jobs, err
}

func (s *Store) MarkNotificationPending(ctx context.Context, jobID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE notification_jobs
		SET status = 'pending'
		WHERE id = $1
	`, jobID)
	return err
}

func (s *Store) MarkNotificationSent(ctx context.Context, jobID string, sentAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE notification_jobs
		SET status = 'sent',
		    sent_at = $2
		WHERE id = $1
	`, jobID, sentAt.UTC())
	return err
}

func (s *Store) MarkNotificationFailed(ctx context.Context, jobID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE notification_jobs
		SET status = 'failed'
		WHERE id = $1
	`, jobID)
	return err
}
