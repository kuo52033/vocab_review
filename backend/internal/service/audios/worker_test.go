package audios

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

func (c fixedClock) Now() time.Time { return c.now }

type fakeRepo struct {
	readyAudio domain.VocabAudio
	hasReady   bool
	jobs       []domain.VocabAudioJob
	completed  []domain.VocabAudioJob
	failed     []domain.VocabAudioJob
}

func (r *fakeRepo) GetReadyVocabAudio(context.Context, string, string, string, float64, string, string) (domain.VocabAudio, bool, error) {
	return r.readyAudio, r.hasReady, nil
}

func (r *fakeRepo) ClaimPendingVocabAudioJobs(context.Context, time.Time, int) ([]domain.VocabAudioJob, error) {
	return r.jobs, nil
}

func (r *fakeRepo) CompleteVocabAudioJob(_ context.Context, job domain.VocabAudioJob, audio domain.VocabAudio) (domain.VocabAudio, bool, error) {
	r.completed = append(r.completed, job)
	r.readyAudio = audio
	return audio, true, nil
}

func (r *fakeRepo) MarkVocabAudioJobFailed(_ context.Context, jobID string, nextAttemptAt time.Time, lastError string) error {
	for _, job := range r.jobs {
		if job.ID == jobID {
			job.NextAttemptAt = nextAttemptAt
			job.LastError = lastError
			r.failed = append(r.failed, job)
		}
	}
	return nil
}

type fakeSpeech struct {
	calls int
	err   error
}

func (s *fakeSpeech) GenerateSpeech(context.Context, domain.VocabAudioJob) ([]byte, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return []byte("mp3"), nil
}

type fakeStorage struct {
	calls int
}

func (s *fakeStorage) Put(context.Context, string, string, []byte) error {
	s.calls++
	return nil
}

func (s *fakeStorage) Bucket() string { return "bucket" }

func TestWorkerReusesReadyAudio(t *testing.T) {
	job := testJob()
	repo := &fakeRepo{hasReady: true, readyAudio: domain.VocabAudio{ID: "aud_existing", Status: "ready"}, jobs: []domain.VocabAudioJob{job}}
	speech := &fakeSpeech{}
	storage := &fakeStorage{}
	worker := NewWorker(repo, speech, storage, fixedClock{now: job.CreatedAt}, slog.Default(), Config{})

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("run once: %v", err)
	}
	if speech.calls != 0 || storage.calls != 0 {
		t.Fatalf("expected no generation/upload, speech=%d storage=%d", speech.calls, storage.calls)
	}
	if len(repo.completed) != 1 {
		t.Fatalf("expected completed job, got %d", len(repo.completed))
	}
}

func TestWorkerGeneratesUploadsAndCompletesAudio(t *testing.T) {
	job := testJob()
	repo := &fakeRepo{jobs: []domain.VocabAudioJob{job}}
	speech := &fakeSpeech{}
	storage := &fakeStorage{}
	worker := NewWorker(repo, speech, storage, fixedClock{now: job.CreatedAt}, slog.Default(), Config{})

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("run once: %v", err)
	}
	if speech.calls != 1 || storage.calls != 1 {
		t.Fatalf("expected generation/upload, speech=%d storage=%d", speech.calls, storage.calls)
	}
	if repo.readyAudio.StorageKey == "" || repo.readyAudio.FileSizeBytes != 3 {
		t.Fatalf("ready audio: %+v", repo.readyAudio)
	}
}

func TestWorkerMarksFailedJobForRetry(t *testing.T) {
	job := testJob()
	job.AttemptCount = 1
	repo := &fakeRepo{jobs: []domain.VocabAudioJob{job}}
	worker := NewWorker(repo, &fakeSpeech{err: errors.New("provider down")}, &fakeStorage{}, fixedClock{now: job.CreatedAt}, slog.Default(), Config{})

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("run once: %v", err)
	}
	if len(repo.failed) != 1 {
		t.Fatalf("expected failed job, got %d", len(repo.failed))
	}
	if !repo.failed[0].NextAttemptAt.Equal(job.CreatedAt.Add(time.Minute)) {
		t.Fatalf("next attempt: got %s", repo.failed[0].NextAttemptAt)
	}
}

func testJob() domain.VocabAudioJob {
	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	return domain.VocabAudioJob{
		ID:            "audjob_1",
		VocabItemID:   "voc_1",
		Provider:      "openai",
		Model:         "gpt-4o-mini-tts",
		Voice:         "alloy",
		Speed:         1,
		OutputFormat:  "mp3",
		InputText:     "serendipity",
		InputHash:     "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Status:        "processing",
		AttemptCount:  1,
		MaxAttempts:   3,
		NextAttemptAt: now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}
