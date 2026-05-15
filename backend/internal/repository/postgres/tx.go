package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"vocabreview/backend/internal/domain"
)

func withTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func insertVocab(ctx context.Context, tx pgx.Tx, item domain.VocabItem) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO vocab_items (
			id, user_id, term, meaning, example_sentence, part_of_speech, source_text, source_url, notes, created_at, updated_at, archived_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, item.ID, item.UserID, item.Term, item.Meaning, item.ExampleSentence, item.PartOfSpeech, item.SourceText, item.SourceURL, item.Notes, item.CreatedAt.UTC(), item.UpdatedAt.UTC(), nullableTime(item.ArchivedAt))
	return err
}

func insertReviewState(ctx context.Context, tx pgx.Tx, state domain.ReviewState) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO review_states (
			vocab_item_id, user_id, status, ease_factor, interval_days, repetition_count, last_reviewed_at, next_due_at, consecutive_again
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, state.VocabItemID, state.UserID, state.Status, state.EaseFactor, state.IntervalDays, state.RepetitionCount, nullableTime(state.LastReviewedAt), state.NextDueAt.UTC(), state.ConsecutiveAgain)
	return err
}

func insertNotificationJob(ctx context.Context, tx pgx.Tx, job *domain.NotificationJob) error {
	if job == nil {
		return nil
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO notification_jobs (id, user_id, vocab_item_id, scheduled_at, sent_at, status, message)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, job.ID, job.UserID, job.VocabItemID, job.ScheduledAt.UTC(), nullableTime(job.SentAt), job.Status, job.Message)
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return nil
	}
	return err
}
