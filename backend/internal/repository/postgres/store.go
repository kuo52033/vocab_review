package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/repository"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*Store, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}
	store := &Store{pool: pool}
	if err := store.HealthCheck(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) HealthCheck(ctx context.Context) error {
	if err := s.pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}

	var versionCount int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM goose_db_version`).Scan(&versionCount); err != nil {
		return fmt.Errorf("schema not ready: %w", err)
	}

	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users)`).Scan(&exists); err != nil {
		return fmt.Errorf("users table unavailable: %w", err)
	}
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM vocab_items)`).Scan(&exists); err != nil {
		return fmt.Errorf("vocab_items table unavailable: %w", err)
	}
	return nil
}

func (s *Store) PutMagicLink(ctx context.Context, token domain.MagicLinkToken) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO magic_links (token, email, expires_at)
		VALUES ($1, $2, $3)
	`, token.Token, token.Email, token.ExpiresAt.UTC())
	return err
}

func (s *Store) ConsumeMagicLink(ctx context.Context, token string, now time.Time, newUser domain.User, newSession domain.Session) (domain.User, domain.Session, error) {
	var user domain.User
	var session domain.Session

	err := withTx(ctx, s.pool, func(tx pgx.Tx) error {
		var email string
		var expiresAt time.Time
		err := tx.QueryRow(ctx, `
			SELECT email, expires_at
			FROM magic_links
			WHERE token = $1
			FOR UPDATE
		`, token).Scan(&email, &expiresAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return repository.ErrNotFound
		}
		if err != nil {
			return err
		}
		if now.After(expiresAt) {
			return repository.ErrExpired
		}

		err = tx.QueryRow(ctx, `
			SELECT id, email, created_at
			FROM users
			WHERE email = $1
		`, email).Scan(&user.ID, &user.Email, &user.CreatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			newUser.Email = email
			if _, err := tx.Exec(ctx, `
				INSERT INTO users (id, email, created_at)
				VALUES ($1, $2, $3)
			`, newUser.ID, newUser.Email, newUser.CreatedAt.UTC()); err != nil {
				return err
			}
			user = newUser
		} else if err != nil {
			return err
		}

		newSession.UserID = user.ID
		if _, err := tx.Exec(ctx, `
			INSERT INTO sessions (token, user_id, created_at, expires_at)
			VALUES ($1, $2, $3, $4)
		`, newSession.Token, newSession.UserID, newSession.CreatedAt.UTC(), newSession.ExpiresAt.UTC()); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM magic_links WHERE token = $1`, token); err != nil {
			return err
		}
		session = newSession
		return nil
	})
	return user, session, err
}

func (s *Store) GetSessionUser(ctx context.Context, token string) (domain.Session, domain.User, bool, error) {
	var session domain.Session
	var user domain.User
	err := s.pool.QueryRow(ctx, `
		SELECT s.token, s.user_id, s.created_at, s.expires_at,
		       u.id, u.email, u.created_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token = $1
	`, token).Scan(
		&session.Token,
		&session.UserID,
		&session.CreatedAt,
		&session.ExpiresAt,
		&user.ID,
		&user.Email,
		&user.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Session{}, domain.User{}, false, nil
	}
	if err != nil {
		return domain.Session{}, domain.User{}, false, err
	}
	return session, user, true, nil
}

func (s *Store) CreateVocab(ctx context.Context, item domain.VocabItem, state domain.ReviewState, job *domain.NotificationJob) error {
	return withTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := insertVocab(ctx, tx, item); err != nil {
			return err
		}
		if err := insertReviewState(ctx, tx, state); err != nil {
			return err
		}
		return insertNotificationJob(ctx, tx, job)
	})
}

func (s *Store) CreateCapturedVocab(ctx context.Context, item domain.VocabItem, state domain.ReviewState, capture domain.CaptureSource, job *domain.NotificationJob) error {
	return withTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := insertVocab(ctx, tx, item); err != nil {
			return err
		}
		if err := insertReviewState(ctx, tx, state); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO capture_sources (
				id, user_id, vocab_item_id, source, selection, page_title, page_url, created_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, capture.ID, capture.UserID, capture.VocabItemID, capture.Source, capture.Selection, capture.PageTitle, capture.PageURL, capture.CreatedAt.UTC()); err != nil {
			return err
		}
		return insertNotificationJob(ctx, tx, job)
	})
}

func (s *Store) GetVocab(ctx context.Context, id string) (domain.VocabItem, bool, error) {
	item, err := scanVocab(
		s.pool.QueryRow(ctx, `
			SELECT id, user_id, term, kind, meaning, example_sentence, source_text, source_url, notes, created_at, updated_at, archived_at
			FROM vocab_items
			WHERE id = $1
		`, id),
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.VocabItem{}, false, nil
	}
	if err != nil {
		return domain.VocabItem{}, false, err
	}
	return item, true, nil
}

func (s *Store) UpdateVocab(ctx context.Context, item domain.VocabItem) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE vocab_items
		SET term = $2,
		    kind = $3,
		    meaning = $4,
		    example_sentence = $5,
		    source_text = $6,
		    source_url = $7,
		    notes = $8,
		    updated_at = $9,
		    archived_at = $10
		WHERE id = $1
	`, item.ID, item.Term, item.Kind, item.Meaning, item.ExampleSentence, item.SourceText, item.SourceURL, item.Notes, item.UpdatedAt.UTC(), nullableTime(item.ArchivedAt))
	return err
}

func (s *Store) ArchiveVocabForUser(ctx context.Context, userID string, vocabID string, archivedAt time.Time) (domain.VocabItem, error) {
	item, err := scanVocab(
		s.pool.QueryRow(ctx, `
			UPDATE vocab_items
			SET updated_at = $3,
			    archived_at = $3
			WHERE id = $1
			  AND user_id = $2
			RETURNING id, user_id, term, kind, meaning, example_sentence, source_text, source_url, notes, created_at, updated_at, archived_at
		`, vocabID, userID, archivedAt.UTC()),
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.VocabItem{}, repository.ErrNotFound
	}
	if err != nil {
		return domain.VocabItem{}, err
	}
	return item, nil
}

func (s *Store) ListVocabByUser(ctx context.Context, userID string) ([]repository.VocabWithState, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			v.id, v.user_id, v.term, v.kind, v.meaning, v.example_sentence, v.source_text, v.source_url, v.notes, v.created_at, v.updated_at, v.archived_at,
			r.vocab_item_id, r.user_id, r.status, r.ease_factor, r.interval_days, r.repetition_count, r.last_reviewed_at, r.next_due_at, r.consecutive_again
		FROM vocab_items v
		JOIN review_states r ON r.vocab_item_id = v.id
		WHERE v.user_id = $1
		  AND v.archived_at IS NULL
		ORDER BY v.created_at ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanVocabWithStates(rows)
}

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

func (s *Store) UpsertDeviceToken(ctx context.Context, token domain.DeviceToken) (domain.DeviceToken, error) {
	var stored domain.DeviceToken
	err := s.pool.QueryRow(ctx, `
		INSERT INTO device_tokens (id, user_id, platform, token, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, token)
		DO UPDATE SET
			platform = EXCLUDED.platform,
			updated_at = EXCLUDED.updated_at
		RETURNING id, user_id, platform, token, created_at, updated_at
	`, token.ID, token.UserID, token.Platform, token.Token, token.CreatedAt.UTC(), token.UpdatedAt.UTC()).Scan(
		&stored.ID,
		&stored.UserID,
		&stored.Platform,
		&stored.Token,
		&stored.CreatedAt,
		&stored.UpdatedAt,
	)
	return stored, err
}

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
			id, user_id, term, kind, meaning, example_sentence, source_text, source_url, notes, created_at, updated_at, archived_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, item.ID, item.UserID, item.Term, item.Kind, item.Meaning, item.ExampleSentence, item.SourceText, item.SourceURL, item.Notes, item.CreatedAt.UTC(), item.UpdatedAt.UTC(), nullableTime(item.ArchivedAt))
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

type scanner interface {
	Scan(dest ...any) error
}

func scanVocab(row scanner) (domain.VocabItem, error) {
	var item domain.VocabItem
	if err := row.Scan(
		&item.ID,
		&item.UserID,
		&item.Term,
		&item.Kind,
		&item.Meaning,
		&item.ExampleSentence,
		&item.SourceText,
		&item.SourceURL,
		&item.Notes,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.ArchivedAt,
	); err != nil {
		return domain.VocabItem{}, err
	}
	return item, nil
}

func scanReviewState(row scanner) (domain.ReviewState, error) {
	var state domain.ReviewState
	if err := row.Scan(
		&state.VocabItemID,
		&state.UserID,
		&state.Status,
		&state.EaseFactor,
		&state.IntervalDays,
		&state.RepetitionCount,
		&state.LastReviewedAt,
		&state.NextDueAt,
		&state.ConsecutiveAgain,
	); err != nil {
		return domain.ReviewState{}, err
	}
	return state, nil
}

func scanVocabWithStates(rows pgx.Rows) ([]repository.VocabWithState, error) {
	result := make([]repository.VocabWithState, 0)
	for rows.Next() {
		var item domain.VocabItem
		var state domain.ReviewState
		if err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.Term,
			&item.Kind,
			&item.Meaning,
			&item.ExampleSentence,
			&item.SourceText,
			&item.SourceURL,
			&item.Notes,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.ArchivedAt,
			&state.VocabItemID,
			&state.UserID,
			&state.Status,
			&state.EaseFactor,
			&state.IntervalDays,
			&state.RepetitionCount,
			&state.LastReviewedAt,
			&state.NextDueAt,
			&state.ConsecutiveAgain,
		); err != nil {
			return nil, err
		}
		result = append(result, repository.VocabWithState{Item: item, State: state})
	}
	return result, rows.Err()
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}
