package notifications

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"vocabreview/backend/internal/domain"
)

type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time {
	return c.now
}

type fakeRepository struct {
	jobs        []domain.NotificationJob
	tokens      map[string][]domain.DeviceToken
	sentJobs    map[string]time.Time
	failedJobs  map[string]bool
	pendingJobs map[string]bool
	claimLimit  int
	claimTime   time.Time
}

func newFakeRepository(jobs []domain.NotificationJob) *fakeRepository {
	return &fakeRepository{
		jobs:        jobs,
		tokens:      map[string][]domain.DeviceToken{},
		sentJobs:    map[string]time.Time{},
		failedJobs:  map[string]bool{},
		pendingJobs: map[string]bool{},
	}
}

func (r *fakeRepository) ClaimDueNotificationJobs(_ context.Context, now time.Time, limit int) ([]domain.NotificationJob, error) {
	r.claimTime = now
	r.claimLimit = limit
	return r.jobs, nil
}

func (r *fakeRepository) ListDeviceTokensForUser(_ context.Context, userID string) ([]domain.DeviceToken, error) {
	return r.tokens[userID], nil
}

func (r *fakeRepository) MarkNotificationSent(_ context.Context, jobID string, sentAt time.Time) error {
	r.sentJobs[jobID] = sentAt
	return nil
}

func (r *fakeRepository) MarkNotificationFailed(_ context.Context, jobID string) error {
	r.failedJobs[jobID] = true
	return nil
}

func (r *fakeRepository) MarkNotificationPending(_ context.Context, jobID string) error {
	r.pendingJobs[jobID] = true
	return nil
}

type fakeSender struct {
	err   error
	sends []sentNotification
}

type sentNotification struct {
	token domain.DeviceToken
	job   domain.NotificationJob
}

func (s *fakeSender) Send(_ context.Context, token domain.DeviceToken, job domain.NotificationJob) error {
	s.sends = append(s.sends, sentNotification{token: token, job: job})
	return s.err
}

func TestWorkerRunOnceDryRunSendsAndMarksSent(t *testing.T) {
	now := time.Date(2026, 5, 9, 8, 0, 0, 0, time.UTC)
	job := domain.NotificationJob{ID: "job_1", UserID: "usr_1", VocabItemID: "voc_1", ScheduledAt: now, Status: "pending", Message: "Review now"}
	repo := newFakeRepository([]domain.NotificationJob{job})
	repo.tokens["usr_1"] = []domain.DeviceToken{{ID: "dev_1", UserID: "usr_1", Platform: "ios", Token: "token_1"}}
	sender := &fakeSender{}
	worker := NewWorker(repo, sender, fixedClock{now: now}, slog.New(slog.NewTextHandler(testWriter{t: t}, nil)), Config{BatchSize: 25})

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("run once: %v", err)
	}

	if repo.claimLimit != 25 || !repo.claimTime.Equal(now) {
		t.Fatalf("claim inputs: limit=%d time=%s", repo.claimLimit, repo.claimTime)
	}
	if len(sender.sends) != 1 || sender.sends[0].job.ID != "job_1" || sender.sends[0].token.ID != "dev_1" {
		t.Fatalf("sends: got %+v", sender.sends)
	}
	if sentAt, ok := repo.sentJobs["job_1"]; !ok || !sentAt.Equal(now) {
		t.Fatalf("sent job: got ok=%v sent_at=%s want %s", ok, sentAt, now)
	}
}

func TestWorkerRunOnceKeepsJobPendingWithoutDeviceTokens(t *testing.T) {
	now := time.Date(2026, 5, 9, 8, 0, 0, 0, time.UTC)
	repo := newFakeRepository([]domain.NotificationJob{{ID: "job_1", UserID: "usr_1", VocabItemID: "voc_1", ScheduledAt: now, Status: "pending", Message: "Review now"}})
	sender := &fakeSender{}
	worker := NewWorker(repo, sender, fixedClock{now: now}, slog.New(slog.NewTextHandler(testWriter{t: t}, nil)), Config{})

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("run once: %v", err)
	}

	if len(sender.sends) != 0 {
		t.Fatalf("expected no sends without device tokens, got %+v", sender.sends)
	}
	if !repo.pendingJobs["job_1"] || len(repo.sentJobs) != 0 || len(repo.failedJobs) != 0 {
		t.Fatalf("expected job reset to pending, pending=%+v sent=%+v failed=%+v", repo.pendingJobs, repo.sentJobs, repo.failedJobs)
	}
}

func TestWorkerRunOnceMarksFailedWhenAllSendsFail(t *testing.T) {
	now := time.Date(2026, 5, 9, 8, 0, 0, 0, time.UTC)
	repo := newFakeRepository([]domain.NotificationJob{{ID: "job_1", UserID: "usr_1", VocabItemID: "voc_1", ScheduledAt: now, Status: "pending", Message: "Review now"}})
	repo.tokens["usr_1"] = []domain.DeviceToken{{ID: "dev_1", UserID: "usr_1", Platform: "ios", Token: "token_1"}}
	sender := &fakeSender{err: errors.New("send failed")}
	worker := NewWorker(repo, sender, fixedClock{now: now}, slog.New(slog.NewTextHandler(testWriter{t: t}, nil)), Config{})

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("run once: %v", err)
	}

	if !repo.failedJobs["job_1"] {
		t.Fatalf("expected job_1 marked failed, failed=%+v", repo.failedJobs)
	}
	if len(repo.sentJobs) != 0 {
		t.Fatalf("expected no sent jobs, got %+v", repo.sentJobs)
	}
}

type testWriter struct {
	t *testing.T
}

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Log(string(p))
	return len(p), nil
}
