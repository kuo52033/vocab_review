package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/repository"
)

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
			SELECT id, user_id, term, kind, meaning, example_sentence, part_of_speech, source_text, source_url, notes, created_at, updated_at, archived_at
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
		    part_of_speech = $6,
		    source_text = $7,
		    source_url = $8,
		    notes = $9,
		    updated_at = $10,
		    archived_at = $11
		WHERE id = $1
	`, item.ID, item.Term, item.Kind, item.Meaning, item.ExampleSentence, item.PartOfSpeech, item.SourceText, item.SourceURL, item.Notes, item.UpdatedAt.UTC(), nullableTime(item.ArchivedAt))
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
			RETURNING id, user_id, term, kind, meaning, example_sentence, part_of_speech, source_text, source_url, notes, created_at, updated_at, archived_at
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

func (s *Store) ListVocabByUser(ctx context.Context, userID string, options repository.ListVocabOptions) ([]repository.VocabWithState, int, error) {
	query := "%" + options.Query + "%"
	status := string(options.Status)
	var total int
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM vocab_items v
		JOIN review_states r ON r.vocab_item_id = v.id
		WHERE v.user_id = $1
		  AND v.archived_at IS NULL
		  AND ($2 = '' OR r.status = $2)
		  AND (
		    $3 = '%%'
		    OR v.term ILIKE $3
		    OR v.meaning ILIKE $3
		    OR v.example_sentence ILIKE $3
		    OR v.notes ILIKE $3
		  )
	`, userID, status, query).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT
			v.id, v.user_id, v.term, v.kind, v.meaning, v.example_sentence, v.part_of_speech, v.source_text, v.source_url, v.notes, v.created_at, v.updated_at, v.archived_at,
			r.vocab_item_id, r.user_id, r.status, r.ease_factor, r.interval_days, r.repetition_count, r.last_reviewed_at, r.next_due_at, r.consecutive_again
		FROM vocab_items v
		JOIN review_states r ON r.vocab_item_id = v.id
		WHERE v.user_id = $1
		  AND v.archived_at IS NULL
		  AND ($2 = '' OR r.status = $2)
		  AND (
		    $3 = '%%'
		    OR v.term ILIKE $3
		    OR v.meaning ILIKE $3
		    OR v.example_sentence ILIKE $3
		    OR v.notes ILIKE $3
		  )
		ORDER BY v.created_at DESC
		LIMIT NULLIF($4, 0)
		OFFSET $5
	`, userID, status, query, options.Limit, options.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items, err := scanVocabWithStates(rows)
	return items, total, err
}
