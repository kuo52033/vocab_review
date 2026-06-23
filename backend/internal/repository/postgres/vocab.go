package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/repository"
)

func (s *Store) CreateVocab(ctx context.Context, item domain.VocabItem, state domain.ReviewState, job *domain.NotificationJob, audioJob *domain.VocabAudioJob) error {
	return withTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := insertVocab(ctx, tx, item); err != nil {
			return err
		}
		if err := insertReviewState(ctx, tx, state); err != nil {
			return err
		}
		if err := insertNotificationJob(ctx, tx, job); err != nil {
			return err
		}
		return upsertVocabAudioJob(ctx, tx, audioJob)
	})
}

func (s *Store) CreateVocabBatch(ctx context.Context, creates []repository.VocabCreate) error {
	return withTx(ctx, s.pool, func(tx pgx.Tx) error {
		for _, create := range creates {
			if err := insertVocab(ctx, tx, create.Item); err != nil {
				return err
			}
			if err := insertReviewState(ctx, tx, create.State); err != nil {
				return err
			}
			if err := insertNotificationJob(ctx, tx, create.Job); err != nil {
				return err
			}
			if err := upsertVocabAudioJob(ctx, tx, create.AudioJob); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) CreateCapturedVocab(ctx context.Context, item domain.VocabItem, state domain.ReviewState, capture domain.CaptureSource, job *domain.NotificationJob, audioJob *domain.VocabAudioJob) error {
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
		if err := insertNotificationJob(ctx, tx, job); err != nil {
			return err
		}
		return upsertVocabAudioJob(ctx, tx, audioJob)
	})
}

func (s *Store) GetVocab(ctx context.Context, id string) (domain.VocabItem, bool, error) {
	item, err := scanVocab(
		s.pool.QueryRow(ctx, `
			SELECT
				v.id, v.user_id, v.term, v.meaning, v.chinese, v.example_sentence, v.part_of_speech, v.source_text, v.source_url, v.notes,
				COALESCE(a.id, ''), COALESCE(a.storage_key, ''), COALESCE(a.status, j.status, ''), COALESCE(a.provider, j.provider, ''), COALESCE(a.model, j.model, ''), COALESCE(a.voice, j.voice, ''), COALESCE(a.speed, j.speed, 0), COALESCE(a.output_format, j.output_format, ''),
				v.created_at, v.updated_at, v.archived_at
			FROM vocab_items v
			LEFT JOIN vocab_audios a ON a.id = v.audio_id
			LEFT JOIN vocab_audio_jobs j ON j.vocab_item_id = v.id
			WHERE v.id = $1
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

func (s *Store) GetActiveVocabByTerm(ctx context.Context, userID string, term string) (repository.VocabWithState, bool, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			v.id, v.user_id, v.term, v.meaning, v.chinese, v.example_sentence, v.part_of_speech, v.source_text, v.source_url, v.notes,
			COALESCE(a.id, ''), COALESCE(a.storage_key, ''), COALESCE(a.status, j.status, ''), COALESCE(a.provider, j.provider, ''), COALESCE(a.model, j.model, ''), COALESCE(a.voice, j.voice, ''), COALESCE(a.speed, j.speed, 0), COALESCE(a.output_format, j.output_format, ''),
			v.created_at, v.updated_at, v.archived_at,
			r.vocab_item_id, r.user_id, r.status, r.ease_factor, r.interval_days, r.repetition_count, r.last_reviewed_at, r.next_due_at, r.consecutive_again
		FROM vocab_items v
		JOIN review_states r ON r.vocab_item_id = v.id
		LEFT JOIN vocab_audios a ON a.id = v.audio_id
		LEFT JOIN vocab_audio_jobs j ON j.vocab_item_id = v.id
		WHERE v.user_id = $1
		  AND v.archived_at IS NULL
		  AND lower(btrim(v.term)) = lower(btrim($2))
		ORDER BY v.created_at DESC
		LIMIT 1
	`, userID, term)
	if err != nil {
		return repository.VocabWithState{}, false, err
	}
	defer rows.Close()
	items, err := scanVocabWithStates(rows)
	if err != nil {
		return repository.VocabWithState{}, false, err
	}
	if len(items) == 0 {
		return repository.VocabWithState{}, false, nil
	}
	return items[0], true, nil
}

func (s *Store) ListActiveVocabByTerms(ctx context.Context, userID string, terms []string) ([]repository.VocabWithState, error) {
	normalized := make([]string, 0, len(terms))
	seen := make(map[string]struct{}, len(terms))
	for _, term := range terms {
		term = strings.ToLower(strings.TrimSpace(term))
		if term == "" {
			continue
		}
		if _, ok := seen[term]; ok {
			continue
		}
		seen[term] = struct{}{}
		normalized = append(normalized, term)
	}
	if len(normalized) == 0 {
		return nil, nil
	}

	rows, err := s.pool.Query(ctx, `
		SELECT
			v.id, v.user_id, v.term, v.meaning, v.chinese, v.example_sentence, v.part_of_speech, v.source_text, v.source_url, v.notes,
			COALESCE(a.id, ''), COALESCE(a.storage_key, ''), COALESCE(a.status, j.status, ''), COALESCE(a.provider, j.provider, ''), COALESCE(a.model, j.model, ''), COALESCE(a.voice, j.voice, ''), COALESCE(a.speed, j.speed, 0), COALESCE(a.output_format, j.output_format, ''),
			v.created_at, v.updated_at, v.archived_at,
			r.vocab_item_id, r.user_id, r.status, r.ease_factor, r.interval_days, r.repetition_count, r.last_reviewed_at, r.next_due_at, r.consecutive_again
		FROM vocab_items v
		JOIN review_states r ON r.vocab_item_id = v.id
		LEFT JOIN vocab_audios a ON a.id = v.audio_id
		LEFT JOIN vocab_audio_jobs j ON j.vocab_item_id = v.id
		WHERE v.user_id = $1
		  AND v.archived_at IS NULL
		  AND lower(btrim(v.term)) = ANY($2::text[])
		ORDER BY v.created_at DESC
	`, userID, normalized)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanVocabWithStates(rows)
}

func (s *Store) UpdateVocab(ctx context.Context, item domain.VocabItem, audioJob *domain.VocabAudioJob) error {
	return withTx(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			UPDATE vocab_items
			SET term = $2,
			    meaning = $3,
			    chinese = $4,
			    example_sentence = $5,
			    part_of_speech = $6,
			    source_text = $7,
			    source_url = $8,
			    notes = $9,
			    audio_id = NULLIF($10, ''),
			    updated_at = $11,
			    archived_at = $12
			WHERE id = $1
		`, item.ID, item.Term, item.Meaning, item.Chinese, item.ExampleSentence, item.PartOfSpeech, item.SourceText, item.SourceURL, item.Notes, item.AudioID, item.UpdatedAt.UTC(), nullableTime(item.ArchivedAt)); err != nil {
			return err
		}
		if audioJob == nil && (item.Audio == nil || item.AudioID != "") {
			if _, err := tx.Exec(ctx, `
				DELETE FROM vocab_audio_jobs
				WHERE vocab_item_id = $1
			`, item.ID); err != nil {
				return err
			}
		}
		return upsertVocabAudioJob(ctx, tx, audioJob)
	})
}

func (s *Store) ArchiveVocabForUser(ctx context.Context, userID string, vocabID string, archivedAt time.Time) (domain.VocabItem, error) {
	item, err := scanVocab(
		s.pool.QueryRow(ctx, `
			UPDATE vocab_items
			SET updated_at = $3,
			    archived_at = $3
			WHERE id = $1
			  AND user_id = $2
			RETURNING id, user_id, term, meaning, chinese, example_sentence, part_of_speech, source_text, source_url, notes, COALESCE(audio_id, ''), '', '', '', '', '', 0::numeric, '', created_at, updated_at, archived_at
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

func (s *Store) ListVocabByUser(ctx context.Context, userID string, options repository.ListVocabOptions) ([]repository.VocabWithState, int, bool, error) {
	query := "%" + options.Query + "%"
	status := string(options.Status)
	queryLimit := options.Limit
	if queryLimit > 0 {
		queryLimit++
	}

	rows, err := s.pool.Query(ctx, `
		SELECT
			v.id, v.user_id, v.term, v.meaning, v.chinese, v.example_sentence, v.part_of_speech, v.source_text, v.source_url, v.notes,
			COALESCE(a.id, ''), COALESCE(a.storage_key, ''), COALESCE(a.status, j.status, ''), COALESCE(a.provider, j.provider, ''), COALESCE(a.model, j.model, ''), COALESCE(a.voice, j.voice, ''), COALESCE(a.speed, j.speed, 0), COALESCE(a.output_format, j.output_format, ''),
			v.created_at, v.updated_at, v.archived_at,
			r.vocab_item_id, r.user_id, r.status, r.ease_factor, r.interval_days, r.repetition_count, r.last_reviewed_at, r.next_due_at, r.consecutive_again
		FROM vocab_items v
		JOIN review_states r ON r.vocab_item_id = v.id
		LEFT JOIN vocab_audios a ON a.id = v.audio_id
		LEFT JOIN vocab_audio_jobs j ON j.vocab_item_id = v.id
		WHERE v.user_id = $1
		  AND v.archived_at IS NULL
		  AND ($2 = '' OR r.status = $2)
		  AND (
		    $3 = '%%'
		    OR v.term ILIKE $3
		    OR v.meaning ILIKE $3
		    OR v.chinese ILIKE $3
		    OR v.example_sentence ILIKE $3
		    OR v.notes ILIKE $3
		)
		ORDER BY v.created_at DESC
		LIMIT NULLIF($4, 0)
		OFFSET $5
	`, userID, status, query, queryLimit, options.Offset)
	if err != nil {
		return nil, 0, false, err
	}
	defer rows.Close()
	items, err := scanVocabWithStates(rows)
	if err != nil {
		return nil, 0, false, err
	}
	hasNext := options.Limit > 0 && len(items) > options.Limit
	if hasNext {
		items = items[:options.Limit]
	}
	total := options.Offset + len(items)
	if hasNext {
		total++
	}
	return items, total, hasNext, nil
}
