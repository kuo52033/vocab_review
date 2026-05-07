package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/repository"
)

func (s *Store) ListDueVocab(ctx context.Context, userID string, now time.Time) ([]repository.VocabWithState, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			v.id, v.user_id, v.term, v.kind, v.meaning, v.example_sentence, v.source_text, v.source_url, v.notes, v.created_at, v.updated_at, v.archived_at,
			r.vocab_item_id, r.user_id, r.status, r.ease_factor, r.interval_days, r.repetition_count, r.last_reviewed_at, r.next_due_at, r.consecutive_again
		FROM review_states r
		JOIN vocab_items v ON v.id = r.vocab_item_id
		WHERE r.user_id = $1
		  AND r.next_due_at <= $2
		  AND v.archived_at IS NULL
		ORDER BY r.next_due_at ASC
	`, userID, now.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanVocabWithStates(rows)
}

func (s *Store) GetReviewState(ctx context.Context, vocabID string) (domain.ReviewState, bool, error) {
	state, err := scanReviewState(
		s.pool.QueryRow(ctx, `
			SELECT vocab_item_id, user_id, status, ease_factor, interval_days, repetition_count, last_reviewed_at, next_due_at, consecutive_again
			FROM review_states
			WHERE vocab_item_id = $1
		`, vocabID),
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ReviewState{}, false, nil
	}
	if err != nil {
		return domain.ReviewState{}, false, err
	}
	return state, true, nil
}

func (s *Store) RecordReview(ctx context.Context, state domain.ReviewState, log domain.ReviewLog, job *domain.NotificationJob) error {
	return withTx(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			UPDATE review_states
			SET status = $2,
			    ease_factor = $3,
			    interval_days = $4,
			    repetition_count = $5,
			    last_reviewed_at = $6,
			    next_due_at = $7,
			    consecutive_again = $8
			WHERE vocab_item_id = $1
		`, state.VocabItemID, state.Status, state.EaseFactor, state.IntervalDays, state.RepetitionCount, nullableTime(state.LastReviewedAt), state.NextDueAt.UTC(), state.ConsecutiveAgain); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO review_logs (id, user_id, vocab_item_id, grade, reviewed_at)
			VALUES ($1, $2, $3, $4, $5)
		`, log.ID, log.UserID, log.VocabItemID, log.Grade, log.ReviewedAt.UTC()); err != nil {
			return err
		}
		return insertNotificationJob(ctx, tx, job)
	})
}

func (s *Store) ListReviewHistory(ctx context.Context, userID string, limit int) ([]repository.ReviewHistoryEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			l.id, l.user_id, l.vocab_item_id, l.grade, l.reviewed_at,
			v.id, v.user_id, v.term, v.kind, v.meaning, v.example_sentence, v.source_text, v.source_url, v.notes, v.created_at, v.updated_at, v.archived_at,
			r.vocab_item_id, r.user_id, r.status, r.ease_factor, r.interval_days, r.repetition_count, r.last_reviewed_at, r.next_due_at, r.consecutive_again
		FROM review_logs l
		JOIN vocab_items v ON v.id = l.vocab_item_id
		JOIN review_states r ON r.vocab_item_id = l.vocab_item_id
		WHERE l.user_id = $1
		ORDER BY l.reviewed_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]repository.ReviewHistoryEntry, 0)
	for rows.Next() {
		var entry repository.ReviewHistoryEntry
		if err := rows.Scan(
			&entry.Log.ID,
			&entry.Log.UserID,
			&entry.Log.VocabItemID,
			&entry.Log.Grade,
			&entry.Log.ReviewedAt,
			&entry.Item.ID,
			&entry.Item.UserID,
			&entry.Item.Term,
			&entry.Item.Kind,
			&entry.Item.Meaning,
			&entry.Item.ExampleSentence,
			&entry.Item.SourceText,
			&entry.Item.SourceURL,
			&entry.Item.Notes,
			&entry.Item.CreatedAt,
			&entry.Item.UpdatedAt,
			&entry.Item.ArchivedAt,
			&entry.State.VocabItemID,
			&entry.State.UserID,
			&entry.State.Status,
			&entry.State.EaseFactor,
			&entry.State.IntervalDays,
			&entry.State.RepetitionCount,
			&entry.State.LastReviewedAt,
			&entry.State.NextDueAt,
			&entry.State.ConsecutiveAgain,
		); err != nil {
			return nil, err
		}
		result = append(result, entry)
	}
	return result, rows.Err()
}

func (s *Store) GetReviewStats(ctx context.Context, userID string, now time.Time) (repository.ReviewStats, error) {
	var stats repository.ReviewStats
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).UTC()
	sevenDaysAgo := now.AddDate(0, 0, -7).UTC()

	err := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE l.reviewed_at >= $2),
			COUNT(*) FILTER (WHERE l.reviewed_at >= $3)
		FROM review_logs l
		WHERE l.user_id = $1
	`, userID, startOfToday, sevenDaysAgo).Scan(&stats.ReviewedToday, &stats.Reviewed7Days)
	if err != nil {
		return repository.ReviewStats{}, err
	}

	err = s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE v.archived_at IS NULL),
			COUNT(*) FILTER (WHERE v.archived_at IS NOT NULL),
			COUNT(*) FILTER (WHERE v.archived_at IS NULL AND r.next_due_at <= $2)
		FROM vocab_items v
		JOIN review_states r ON r.vocab_item_id = v.id
		WHERE v.user_id = $1
	`, userID, now.UTC()).Scan(&stats.ActiveCards, &stats.ArchivedCards, &stats.DueNow)
	if err != nil {
		return repository.ReviewStats{}, err
	}

	return stats, nil
}
