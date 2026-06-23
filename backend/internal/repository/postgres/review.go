package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/repository"
)

func (s *Store) ListDueVocab(ctx context.Context, userID string, now time.Time, limit int) ([]repository.VocabWithState, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			v.id, v.user_id, v.term, v.meaning, v.chinese, v.example_sentence, v.part_of_speech, v.source_text, v.source_url, v.notes,
			COALESCE(a.id, ''), COALESCE(a.storage_key, ''), COALESCE(a.status, j.status, ''), COALESCE(a.provider, j.provider, ''), COALESCE(a.model, j.model, ''), COALESCE(a.voice, j.voice, ''), COALESCE(a.speed, j.speed, 0), COALESCE(a.output_format, j.output_format, ''),
			v.created_at, v.updated_at, v.archived_at,
			r.vocab_item_id, r.user_id, r.status, r.ease_factor, r.interval_days, r.repetition_count, r.last_reviewed_at, r.next_due_at, r.consecutive_again
		FROM review_states r
		JOIN vocab_items v ON v.id = r.vocab_item_id
		LEFT JOIN vocab_audios a ON a.id = v.audio_id
		LEFT JOIN vocab_audio_jobs j ON j.vocab_item_id = v.id
		WHERE r.user_id = $1
		  AND r.next_due_at <= $2
		  AND v.archived_at IS NULL
		ORDER BY r.next_due_at ASC
		LIMIT NULLIF($3, 0)
	`, userID, now.UTC(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanVocabWithStates(rows)
}

func (s *Store) ListReviewSessionCandidates(ctx context.Context, userID string, limit int) ([]repository.ReviewSessionCandidate, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, term, meaning, chinese
		FROM vocab_items
		WHERE user_id = $1
		  AND archived_at IS NULL
		  AND meaning <> ''
		ORDER BY created_at DESC
		LIMIT NULLIF($2, 0)
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanReviewSessionCandidates(rows)
}

func scanReviewSessionCandidates(rows pgx.Rows) ([]repository.ReviewSessionCandidate, error) {
	candidates := make([]repository.ReviewSessionCandidate, 0)
	for rows.Next() {
		var candidate repository.ReviewSessionCandidate
		if err := rows.Scan(&candidate.ID, &candidate.Term, &candidate.Meaning, &candidate.Chinese); err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return candidates, nil
}

func reviewStatsSQL() string {
	return `
		SELECT
			COALESCE(log_counts.reviewed_today, 0),
			COALESCE(log_counts.reviewed_7_days, 0),
			COALESCE(card_counts.active_cards, 0),
			COALESCE(card_counts.archived_cards, 0),
			COALESCE(card_counts.due_now, 0)
		FROM (
			SELECT
				COUNT(*) FILTER (WHERE reviewed_at >= $2) AS reviewed_today,
				COUNT(*) FILTER (WHERE reviewed_at >= $3) AS reviewed_7_days
			FROM review_logs
			WHERE user_id = $1
		) log_counts
		CROSS JOIN (
			SELECT
				COUNT(*) FILTER (WHERE v.archived_at IS NULL) AS active_cards,
				COUNT(*) FILTER (WHERE v.archived_at IS NOT NULL) AS archived_cards,
				COUNT(*) FILTER (WHERE v.archived_at IS NULL AND r.next_due_at <= $4) AS due_now
			FROM vocab_items v
			JOIN review_states r ON r.vocab_item_id = v.id
			WHERE v.user_id = $1
		) card_counts
	`
}

func (s *Store) GetReviewSessionData(ctx context.Context, userID string, now time.Time, dueLimit int, candidateLimit int) (repository.ReviewSessionData, error) {
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).UTC()
	sevenDaysAgo := now.AddDate(0, 0, -7).UTC()

	batch := &pgx.Batch{}
	batch.Queue(`
		SELECT
			v.id, v.user_id, v.term, v.meaning, v.chinese, v.example_sentence, v.part_of_speech, v.source_text, v.source_url, v.notes,
			COALESCE(a.id, ''), COALESCE(a.storage_key, ''), COALESCE(a.status, j.status, ''), COALESCE(a.provider, j.provider, ''), COALESCE(a.model, j.model, ''), COALESCE(a.voice, j.voice, ''), COALESCE(a.speed, j.speed, 0), COALESCE(a.output_format, j.output_format, ''),
			v.created_at, v.updated_at, v.archived_at,
			r.vocab_item_id, r.user_id, r.status, r.ease_factor, r.interval_days, r.repetition_count, r.last_reviewed_at, r.next_due_at, r.consecutive_again
		FROM review_states r
		JOIN vocab_items v ON v.id = r.vocab_item_id
		LEFT JOIN vocab_audios a ON a.id = v.audio_id
		LEFT JOIN vocab_audio_jobs j ON j.vocab_item_id = v.id
		WHERE r.user_id = $1
		  AND r.next_due_at <= $2
		  AND v.archived_at IS NULL
		ORDER BY r.next_due_at ASC
		LIMIT NULLIF($3, 0)
	`, userID, now.UTC(), dueLimit)
	batch.Queue(reviewStatsSQL(), userID, startOfToday, sevenDaysAgo, now.UTC())
	batch.Queue(`
		SELECT id, term, meaning, chinese
		FROM vocab_items
		WHERE user_id = $1
		  AND archived_at IS NULL
		  AND meaning <> ''
		ORDER BY created_at DESC
		LIMIT NULLIF($2, 0)
	`, userID, candidateLimit)

	results := s.pool.SendBatch(ctx, batch)
	defer results.Close()

	dueRows, err := results.Query()
	if err != nil {
		return repository.ReviewSessionData{}, err
	}
	due, err := scanVocabWithStates(dueRows)
	dueRows.Close()
	if err != nil {
		return repository.ReviewSessionData{}, err
	}

	var stats repository.ReviewStats
	if err := results.QueryRow().Scan(&stats.ReviewedToday, &stats.Reviewed7Days, &stats.ActiveCards, &stats.ArchivedCards, &stats.DueNow); err != nil {
		return repository.ReviewSessionData{}, err
	}

	candidateRows, err := results.Query()
	if err != nil {
		return repository.ReviewSessionData{}, err
	}
	candidates, err := scanReviewSessionCandidates(candidateRows)
	candidateRows.Close()
	if err != nil {
		return repository.ReviewSessionData{}, err
	}

	return repository.ReviewSessionData{Due: due, Candidates: candidates, Stats: stats}, nil
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
		batch := &pgx.Batch{}
		batch.Queue(`
			UPDATE review_states
			SET status = $2,
			    ease_factor = $3,
			    interval_days = $4,
			    repetition_count = $5,
			    last_reviewed_at = $6,
			    next_due_at = $7,
			    consecutive_again = $8
			WHERE vocab_item_id = $1
		`, state.VocabItemID, state.Status, state.EaseFactor, state.IntervalDays, state.RepetitionCount, nullableTime(state.LastReviewedAt), state.NextDueAt.UTC(), state.ConsecutiveAgain)
		batch.Queue(`
			INSERT INTO review_logs (id, user_id, vocab_item_id, grade, reviewed_at)
			VALUES ($1, $2, $3, $4, $5)
		`, log.ID, log.UserID, log.VocabItemID, log.Grade, log.ReviewedAt.UTC())
		if job != nil {
			batch.Queue(`
				INSERT INTO notification_jobs (id, user_id, vocab_item_id, scheduled_at, sent_at, status, message)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
				ON CONFLICT (id) DO NOTHING
			`, job.ID, job.UserID, job.VocabItemID, job.ScheduledAt.UTC(), nullableTime(job.SentAt), job.Status, job.Message)
		}

		results := tx.SendBatch(ctx, batch)
		defer results.Close()
		for i := 0; i < batch.Len(); i++ {
			if _, err := results.Exec(); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) ListReviewHistory(ctx context.Context, userID string, pagination repository.Pagination) ([]repository.ReviewHistoryEntry, int, bool, error) {
	queryLimit := pagination.Limit
	if queryLimit > 0 {
		queryLimit++
	}
	rows, err := s.pool.Query(ctx, `
		SELECT
			l.id, l.user_id, l.vocab_item_id, l.grade, l.reviewed_at,
			v.id, v.user_id, v.term, v.meaning, v.chinese, v.example_sentence, v.part_of_speech, v.source_text, v.source_url, v.notes,
			COALESCE(a.id, ''), COALESCE(a.storage_key, ''), COALESCE(a.status, j.status, ''), COALESCE(a.provider, j.provider, ''), COALESCE(a.model, j.model, ''), COALESCE(a.voice, j.voice, ''), COALESCE(a.speed, j.speed, 0), COALESCE(a.output_format, j.output_format, ''),
			v.created_at, v.updated_at, v.archived_at,
			r.vocab_item_id, r.user_id, r.status, r.ease_factor, r.interval_days, r.repetition_count, r.last_reviewed_at, r.next_due_at, r.consecutive_again
		FROM review_logs l
		JOIN vocab_items v ON v.id = l.vocab_item_id
		JOIN review_states r ON r.vocab_item_id = l.vocab_item_id
		LEFT JOIN vocab_audios a ON a.id = v.audio_id
		LEFT JOIN vocab_audio_jobs j ON j.vocab_item_id = v.id
		WHERE l.user_id = $1
		ORDER BY l.reviewed_at DESC
		LIMIT NULLIF($2, 0)
		OFFSET $3
	`, userID, queryLimit, pagination.Offset)
	if err != nil {
		return nil, 0, false, err
	}
	defer rows.Close()

	result := make([]repository.ReviewHistoryEntry, 0)
	for rows.Next() {
		var entry repository.ReviewHistoryEntry
		var audioStorageKey, audioStatus, audioProvider, audioModel, audioVoice, audioFormat string
		var audioSpeed float64
		if err := rows.Scan(
			&entry.Log.ID,
			&entry.Log.UserID,
			&entry.Log.VocabItemID,
			&entry.Log.Grade,
			&entry.Log.ReviewedAt,
			&entry.Item.ID,
			&entry.Item.UserID,
			&entry.Item.Term,
			&entry.Item.Meaning,
			&entry.Item.Chinese,
			&entry.Item.ExampleSentence,
			&entry.Item.PartOfSpeech,
			&entry.Item.SourceText,
			&entry.Item.SourceURL,
			&entry.Item.Notes,
			&entry.Item.AudioID,
			&audioStorageKey,
			&audioStatus,
			&audioProvider,
			&audioModel,
			&audioVoice,
			&audioSpeed,
			&audioFormat,
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
			return nil, 0, false, err
		}
		entry.Item.Audio = audioFromScan(entry.Item.AudioID, audioStorageKey, audioStatus, audioProvider, audioModel, audioVoice, audioSpeed, audioFormat)
		result = append(result, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, false, err
	}
	hasNext := pagination.Limit > 0 && len(result) > pagination.Limit
	if hasNext {
		result = result[:pagination.Limit]
	}
	total := pagination.Offset + len(result)
	if hasNext {
		total++
	}
	return result, total, hasNext, nil
}

func (s *Store) GetReviewStats(ctx context.Context, userID string, now time.Time) (repository.ReviewStats, error) {
	var stats repository.ReviewStats
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).UTC()
	sevenDaysAgo := now.AddDate(0, 0, -7).UTC()

	err := s.pool.QueryRow(ctx, reviewStatsSQL(), userID, startOfToday, sevenDaysAgo, now.UTC()).Scan(&stats.ReviewedToday, &stats.Reviewed7Days, &stats.ActiveCards, &stats.ArchivedCards, &stats.DueNow)
	if err != nil {
		return repository.ReviewStats{}, err
	}

	return stats, nil
}
