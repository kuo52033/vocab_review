package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"vocabreview/backend/internal/domain"
)

func (s *Store) ListVocabItemsMissingAudio(ctx context.Context, limit int) ([]domain.VocabItem, error) {
	query := `
		SELECT id, user_id, term, meaning, chinese, example_sentence, part_of_speech, source_text, source_url, notes, COALESCE(audio_id, ''), '', '', '', '', '', 0::numeric, '', created_at, updated_at, archived_at
		FROM vocab_items v
		WHERE v.archived_at IS NULL
		  AND v.audio_id IS NULL
		  AND NOT EXISTS (
		    SELECT 1
		    FROM vocab_audio_jobs j
		    WHERE j.vocab_item_id = v.id
		  )
		ORDER BY v.created_at ASC
	`
	args := []any{}
	if limit > 0 {
		query += " LIMIT $1"
		args = append(args, limit)
	}
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.VocabItem, 0)
	for rows.Next() {
		item, err := scanVocab(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) AttachReadyVocabAudio(ctx context.Context, vocabItemID, audioID string, now time.Time) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE vocab_items
		SET audio_id = $2,
		    updated_at = $3
		WHERE id = $1
		  AND audio_id IS NULL
		  AND archived_at IS NULL
		  AND NOT EXISTS (
		    SELECT 1
		    FROM vocab_audio_jobs j
		    WHERE j.vocab_item_id = vocab_items.id
		  )
	`, vocabItemID, audioID, now.UTC())
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s *Store) EnqueueVocabAudioJob(ctx context.Context, job domain.VocabAudioJob) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO vocab_audio_jobs (
			id, vocab_item_id, provider, model, voice, speed, output_format, input_text, input_hash,
			status, attempt_count, max_attempts, next_attempt_at, last_error, audio_id, created_at, updated_at
		)
		SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, NULLIF($14, ''), NULLIF($15, ''), $16, $17
		WHERE EXISTS (
			SELECT 1
			FROM vocab_items v
			WHERE v.id = $2
			  AND v.audio_id IS NULL
			  AND v.archived_at IS NULL
		)
		ON CONFLICT (vocab_item_id) DO NOTHING
	`, job.ID, job.VocabItemID, job.Provider, job.Model, job.Voice, job.Speed, job.OutputFormat, job.InputText, job.InputHash, job.Status, job.AttemptCount, job.MaxAttempts, job.NextAttemptAt.UTC(), job.LastError, job.AudioID, job.CreatedAt.UTC(), job.UpdatedAt.UTC())
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s *Store) GetReadyVocabAudio(ctx context.Context, provider, model, voice string, speed float64, outputFormat, inputHash string) (domain.VocabAudio, bool, error) {
	audio, err := scanVocabAudio(s.pool.QueryRow(ctx, `
		SELECT id, provider, model, voice, speed, output_format, input_text, input_hash, storage_provider, storage_bucket, storage_key, content_type, COALESCE(file_size_bytes, 0), duration_ms, status, COALESCE(error_message, ''), created_at, updated_at
		FROM vocab_audios
		WHERE provider = $1
		  AND model = $2
		  AND voice = $3
		  AND speed = $4
		  AND output_format = $5
		  AND input_hash = $6
		  AND status = 'ready'
	`, provider, model, voice, speed, outputFormat, inputHash))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.VocabAudio{}, false, nil
	}
	if err != nil {
		return domain.VocabAudio{}, false, err
	}
	return audio, true, nil
}

func (s *Store) ClaimPendingVocabAudioJobs(ctx context.Context, now time.Time, limit int) ([]domain.VocabAudioJob, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.pool.Query(ctx, `
		WITH candidates AS (
			SELECT j.id
			FROM vocab_audio_jobs j
			WHERE j.status = 'pending'
			  AND j.next_attempt_at <= $1
			  AND j.attempt_count < j.max_attempts
			  AND NOT EXISTS (
			    SELECT 1
			    FROM vocab_audio_jobs processing
			    WHERE processing.status = 'processing'
			      AND processing.provider = j.provider
			      AND processing.model = j.model
			      AND processing.voice = j.voice
			      AND processing.speed = j.speed
			      AND processing.output_format = j.output_format
			      AND processing.input_hash = j.input_hash
			  )
			ORDER BY j.created_at ASC
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		UPDATE vocab_audio_jobs j
		SET status = 'processing',
		    attempt_count = j.attempt_count + 1,
		    updated_at = $1
		FROM candidates
		WHERE j.id = candidates.id
		RETURNING j.id, j.vocab_item_id, j.provider, j.model, j.voice, j.speed, j.output_format, j.input_text, j.input_hash, j.status, j.attempt_count, j.max_attempts, j.next_attempt_at, COALESCE(j.last_error, ''), COALESCE(j.audio_id, ''), j.created_at, j.updated_at
	`, now.UTC(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := make([]domain.VocabAudioJob, 0)
	for rows.Next() {
		job, err := scanVocabAudioJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (s *Store) CompleteVocabAudioJob(ctx context.Context, job domain.VocabAudioJob, audio domain.VocabAudio) (domain.VocabAudio, bool, error) {
	var completed domain.VocabAudio
	attached := false
	err := withTx(ctx, s.pool, func(tx pgx.Tx) error {
		existing, err := scanVocabAudio(tx.QueryRow(ctx, `
			INSERT INTO vocab_audios (
				id, provider, model, voice, speed, output_format, input_text, input_hash,
				storage_provider, storage_bucket, storage_key, content_type, file_size_bytes, duration_ms,
				status, error_message, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, 'ready', NULLIF($15, ''), $16, $17)
			ON CONFLICT (provider, model, voice, speed, output_format, input_hash) DO UPDATE
			SET updated_at = vocab_audios.updated_at
			RETURNING id, provider, model, voice, speed, output_format, input_text, input_hash, storage_provider, storage_bucket, storage_key, content_type, COALESCE(file_size_bytes, 0), duration_ms, status, COALESCE(error_message, ''), created_at, updated_at
		`, audio.ID, audio.Provider, audio.Model, audio.Voice, audio.Speed, audio.OutputFormat, audio.InputText, audio.InputHash, audio.StorageProvider, audio.StorageBucket, audio.StorageKey, audio.ContentType, audio.FileSizeBytes, audio.DurationMS, "", audio.CreatedAt.UTC(), audio.UpdatedAt.UTC()))
		if err != nil {
			return err
		}
		completed = existing

		tag, err := tx.Exec(ctx, `
			UPDATE vocab_items v
			SET audio_id = $3,
			    updated_at = $4
			FROM vocab_audio_jobs j
			WHERE j.id = $1
			  AND j.vocab_item_id = $2
			  AND v.id = j.vocab_item_id
			  AND j.status = 'processing'
		`, job.ID, job.VocabItemID, existing.ID, time.Now().UTC())
		if err != nil {
			return err
		}
		attached = tag.RowsAffected() > 0
		if !attached {
			return nil
		}
		_, err = tx.Exec(ctx, `
			UPDATE vocab_audio_jobs
			SET status = 'ready',
			    audio_id = $3,
			    last_error = NULL,
			    updated_at = $4
			WHERE id = $1
			  AND vocab_item_id = $2
		`, job.ID, job.VocabItemID, existing.ID, time.Now().UTC())
		return err
	})
	return completed, attached, err
}

func (s *Store) MarkVocabAudioJobFailed(ctx context.Context, jobID string, nextAttemptAt time.Time, lastError string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE vocab_audio_jobs
		SET status = CASE WHEN attempt_count < max_attempts THEN 'pending' ELSE 'failed' END,
		    next_attempt_at = $2,
		    last_error = $3,
		    updated_at = $2
		WHERE id = $1
		  AND status = 'processing'
	`, jobID, nextAttemptAt.UTC(), lastError)
	return err
}

func scanVocabAudio(row scanner) (domain.VocabAudio, error) {
	var audio domain.VocabAudio
	var errorMessage string
	if err := row.Scan(
		&audio.ID,
		&audio.Provider,
		&audio.Model,
		&audio.Voice,
		&audio.Speed,
		&audio.OutputFormat,
		&audio.InputText,
		&audio.InputHash,
		&audio.StorageProvider,
		&audio.StorageBucket,
		&audio.StorageKey,
		&audio.ContentType,
		&audio.FileSizeBytes,
		&audio.DurationMS,
		&audio.Status,
		&errorMessage,
		&audio.CreatedAt,
		&audio.UpdatedAt,
	); err != nil {
		return domain.VocabAudio{}, err
	}
	return audio, nil
}

func scanVocabAudioJob(row scanner) (domain.VocabAudioJob, error) {
	var job domain.VocabAudioJob
	if err := row.Scan(
		&job.ID,
		&job.VocabItemID,
		&job.Provider,
		&job.Model,
		&job.Voice,
		&job.Speed,
		&job.OutputFormat,
		&job.InputText,
		&job.InputHash,
		&job.Status,
		&job.AttemptCount,
		&job.MaxAttempts,
		&job.NextAttemptAt,
		&job.LastError,
		&job.AudioID,
		&job.CreatedAt,
		&job.UpdatedAt,
	); err != nil {
		return domain.VocabAudioJob{}, err
	}
	return job, nil
}
