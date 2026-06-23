package audios

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"vocabreview/backend/internal/clock"
	"vocabreview/backend/internal/domain"
)

const defaultBatchSize = 10

type Repository interface {
	GetReadyVocabAudio(ctx context.Context, provider, model, voice string, speed float64, outputFormat, inputHash string) (domain.VocabAudio, bool, error)
	ClaimPendingVocabAudioJobs(ctx context.Context, now time.Time, limit int) ([]domain.VocabAudioJob, error)
	CompleteVocabAudioJob(ctx context.Context, job domain.VocabAudioJob, audio domain.VocabAudio) (domain.VocabAudio, bool, error)
	MarkVocabAudioJobFailed(ctx context.Context, jobID string, nextAttemptAt time.Time, lastError string) error
}

type SpeechGenerator interface {
	GenerateSpeech(ctx context.Context, job domain.VocabAudioJob) ([]byte, error)
}

type Storage interface {
	Put(ctx context.Context, key, contentType string, data []byte) error
	Bucket() string
}

type Config struct {
	BatchSize int
}

type Worker struct {
	repo      Repository
	speech    SpeechGenerator
	storage   Storage
	clock     clock.Clock
	logger    *slog.Logger
	batchSize int
}

func NewWorker(repo Repository, speech SpeechGenerator, storage Storage, appClock clock.Clock, logger *slog.Logger, config Config) *Worker {
	batchSize := config.BatchSize
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Worker{repo: repo, speech: speech, storage: storage, clock: appClock, logger: logger, batchSize: batchSize}
}

func (w *Worker) Run(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	if err := w.Drain(ctx); err != nil {
		return err
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := w.Drain(ctx); err != nil {
				w.logger.Error("audio worker drain failed", "error", err)
			}
		}
	}
}

func (w *Worker) RunOnce(ctx context.Context) error {
	_, err := w.RunBatch(ctx)
	return err
}

func (w *Worker) Drain(ctx context.Context) error {
	for {
		claimed, err := w.RunBatch(ctx)
		if err != nil {
			return err
		}
		if claimed == 0 {
			return nil
		}
	}
}

func (w *Worker) RunBatch(ctx context.Context) (int, error) {
	now := w.clock.Now()
	jobs, err := w.repo.ClaimPendingVocabAudioJobs(ctx, now, w.batchSize)
	if err != nil {
		return 0, err
	}
	w.logger.Info("audio batch claimed", "jobs", len(jobs), "batch_size", w.batchSize)
	for _, job := range jobs {
		if err := w.processJob(ctx, job, now); err != nil {
			return len(jobs), err
		}
	}
	return len(jobs), nil
}

func (w *Worker) processJob(ctx context.Context, job domain.VocabAudioJob, now time.Time) error {
	audio, ok, err := w.repo.GetReadyVocabAudio(ctx, job.Provider, job.Model, job.Voice, job.Speed, job.OutputFormat, job.InputHash)
	if err != nil {
		return err
	}
	if ok {
		_, _, err := w.repo.CompleteVocabAudioJob(ctx, job, audio)
		return err
	}

	mp3, err := w.speech.GenerateSpeech(ctx, job)
	if err != nil {
		return w.failJob(ctx, job, now, fmt.Errorf("generate speech: %w", err))
	}
	key := storageKey(job)
	if err := w.storage.Put(ctx, key, "audio/mpeg", mp3); err != nil {
		return w.failJob(ctx, job, now, fmt.Errorf("upload audio: %w", err))
	}

	ready := domain.VocabAudio{
		ID:              audioID(job.InputHash),
		Provider:        job.Provider,
		Model:           job.Model,
		Voice:           job.Voice,
		Speed:           job.Speed,
		OutputFormat:    job.OutputFormat,
		InputText:       job.InputText,
		InputHash:       job.InputHash,
		StorageProvider: "s3",
		StorageBucket:   w.storage.Bucket(),
		StorageKey:      key,
		ContentType:     "audio/mpeg",
		FileSizeBytes:   int64(len(mp3)),
		Status:          "ready",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if _, _, err := w.repo.CompleteVocabAudioJob(ctx, job, ready); err != nil {
		return err
	}
	return nil
}

func (w *Worker) failJob(ctx context.Context, job domain.VocabAudioJob, now time.Time, err error) error {
	delay := retryDelay(job.AttemptCount)
	if markErr := w.repo.MarkVocabAudioJobFailed(ctx, job.ID, now.Add(delay), safeError(err)); markErr != nil {
		return errors.Join(err, markErr)
	}
	w.logger.Error("audio job failed", "job_id", job.ID, "vocab_item_id", job.VocabItemID, "attempt", job.AttemptCount, "error", err)
	return nil
}

func retryDelay(attempt int) time.Duration {
	switch attempt {
	case 0, 1:
		return time.Minute
	case 2:
		return 5 * time.Minute
	default:
		return 30 * time.Minute
	}
}

func safeError(err error) string {
	message := strings.TrimSpace(err.Error())
	if len(message) > 1000 {
		return message[:1000]
	}
	return message
}
