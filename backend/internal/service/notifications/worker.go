package notifications

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"vocabreview/backend/internal/clock"
	"vocabreview/backend/internal/domain"
)

const defaultBatchSize = 50

type Repository interface {
	ClaimDueNotificationJobs(ctx context.Context, now time.Time, limit int) ([]domain.NotificationJob, error)
	ListDeviceTokensForUser(ctx context.Context, userID string) ([]domain.DeviceToken, error)
	MarkNotificationPending(ctx context.Context, jobID string) error
	MarkNotificationSent(ctx context.Context, jobID string, sentAt time.Time) error
	MarkNotificationFailed(ctx context.Context, jobID string) error
}

type Sender interface {
	Send(ctx context.Context, token domain.DeviceToken, job domain.NotificationJob) error
}

type Config struct {
	BatchSize int
}

type Worker struct {
	repo      Repository
	sender    Sender
	clock     clock.Clock
	logger    *slog.Logger
	batchSize int
}

func NewWorker(repo Repository, sender Sender, appClock clock.Clock, logger *slog.Logger, config Config) *Worker {
	batchSize := config.BatchSize
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Worker{
		repo:      repo,
		sender:    sender,
		clock:     appClock,
		logger:    logger,
		batchSize: batchSize,
	}
}

func (w *Worker) Run(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	if err := w.RunOnce(ctx); err != nil {
		return err
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := w.RunOnce(ctx); err != nil {
				w.logger.Error("notification worker batch failed", "error", err)
			}
		}
	}
}

func (w *Worker) RunOnce(ctx context.Context) error {
	now := w.clock.Now()
	jobs, err := w.repo.ClaimDueNotificationJobs(ctx, now, w.batchSize)
	if err != nil {
		return err
	}
	w.logger.Info("notification batch claimed", "jobs", len(jobs), "batch_size", w.batchSize)

	for _, job := range jobs {
		if err := w.processJob(ctx, job, now); err != nil {
			return err
		}
	}
	return nil
}

func (w *Worker) processJob(ctx context.Context, job domain.NotificationJob, now time.Time) error {
	tokens, err := w.repo.ListDeviceTokensForUser(ctx, job.UserID)
	if err != nil {
		return err
	}
	if len(tokens) == 0 {
		w.logger.Info("notification job has no device tokens", "job_id", job.ID, "user_id", job.UserID, "vocab_item_id", job.VocabItemID)
		return w.repo.MarkNotificationPending(ctx, job.ID)
	}

	successes := 0
	var sendErr error
	for _, token := range tokens {
		if err := w.sender.Send(ctx, token, job); err != nil {
			sendErr = errors.Join(sendErr, err)
			w.logger.Error("notification send failed", "job_id", job.ID, "device_id", token.ID, "platform", token.Platform, "error", err)
			continue
		}
		successes++
	}

	if successes > 0 {
		return w.repo.MarkNotificationSent(ctx, job.ID, now)
	}
	if err := w.repo.MarkNotificationFailed(ctx, job.ID); err != nil {
		return err
	}
	if sendErr != nil {
		w.logger.Error("notification job failed for all device tokens", "job_id", job.ID, "error", sendErr)
	}
	return nil
}

type DryRunSender struct {
	Logger *slog.Logger
}

func (s DryRunSender) Send(_ context.Context, token domain.DeviceToken, job domain.NotificationJob) error {
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info(
		"notification_dry_run",
		"job_id", job.ID,
		"user_id", job.UserID,
		"vocab_item_id", job.VocabItemID,
		"device_id", token.ID,
		"platform", token.Platform,
		"scheduled_at", job.ScheduledAt,
		"message", job.Message,
	)
	return nil
}
