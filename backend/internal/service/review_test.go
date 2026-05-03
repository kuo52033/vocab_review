package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/repository"
)

type stubClock struct {
	now time.Time
}

func (s stubClock) Now() time.Time { return s.now }

type fakeRepository struct {
	users            map[string]domain.User
	usersByEmail     map[string]string
	sessions         map[string]domain.Session
	magicLinks       map[string]domain.MagicLinkToken
	vocab            map[string]domain.VocabItem
	reviewStates     map[string]domain.ReviewState
	reviewLogs       map[string]domain.ReviewLog
	captures         map[string]domain.CaptureSource
	deviceTokens     map[string]domain.DeviceToken
	notificationJobs map[string]domain.NotificationJob
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		users:            map[string]domain.User{},
		usersByEmail:     map[string]string{},
		sessions:         map[string]domain.Session{},
		magicLinks:       map[string]domain.MagicLinkToken{},
		vocab:            map[string]domain.VocabItem{},
		reviewStates:     map[string]domain.ReviewState{},
		reviewLogs:       map[string]domain.ReviewLog{},
		captures:         map[string]domain.CaptureSource{},
		deviceTokens:     map[string]domain.DeviceToken{},
		notificationJobs: map[string]domain.NotificationJob{},
	}
}

func (f *fakeRepository) HealthCheck(context.Context) error { return nil }

func (f *fakeRepository) PutMagicLink(_ context.Context, token domain.MagicLinkToken) error {
	f.magicLinks[token.Token] = token
	return nil
}

func (f *fakeRepository) ConsumeMagicLink(_ context.Context, token string, now time.Time, newUser domain.User, newSession domain.Session) (domain.User, domain.Session, error) {
	link, ok := f.magicLinks[token]
	if !ok {
		return domain.User{}, domain.Session{}, repository.ErrNotFound
	}
	if now.After(link.ExpiresAt) {
		return domain.User{}, domain.Session{}, repository.ErrExpired
	}

	var user domain.User
	if userID, exists := f.usersByEmail[link.Email]; exists {
		user = f.users[userID]
	} else {
		newUser.Email = link.Email
		user = newUser
		f.users[user.ID] = user
		f.usersByEmail[user.Email] = user.ID
	}

	newSession.UserID = user.ID
	f.sessions[newSession.Token] = newSession
	delete(f.magicLinks, token)
	return user, newSession, nil
}

func (f *fakeRepository) GetSessionUser(_ context.Context, token string) (domain.Session, domain.User, bool, error) {
	session, ok := f.sessions[token]
	if !ok {
		return domain.Session{}, domain.User{}, false, nil
	}
	user, ok := f.users[session.UserID]
	if !ok {
		return domain.Session{}, domain.User{}, false, errors.New("user not found")
	}
	return session, user, true, nil
}

func (f *fakeRepository) CreateVocab(_ context.Context, item domain.VocabItem, state domain.ReviewState, job *domain.NotificationJob) error {
	f.vocab[item.ID] = item
	f.reviewStates[state.VocabItemID] = state
	if job != nil {
		f.notificationJobs[job.ID] = *job
	}
	return nil
}

func (f *fakeRepository) CreateCapturedVocab(_ context.Context, item domain.VocabItem, state domain.ReviewState, capture domain.CaptureSource, job *domain.NotificationJob) error {
	if err := f.CreateVocab(context.Background(), item, state, job); err != nil {
		return err
	}
	f.captures[capture.ID] = capture
	return nil
}

func (f *fakeRepository) GetVocab(_ context.Context, id string) (domain.VocabItem, bool, error) {
	item, ok := f.vocab[id]
	return item, ok, nil
}

func (f *fakeRepository) UpdateVocab(_ context.Context, item domain.VocabItem) error {
	f.vocab[item.ID] = item
	return nil
}

func (f *fakeRepository) ListVocabByUser(_ context.Context, userID string) ([]repository.VocabWithState, error) {
	items := make([]repository.VocabWithState, 0)
	for _, item := range f.vocab {
		if item.UserID != userID {
			continue
		}
		items = append(items, repository.VocabWithState{Item: item, State: f.reviewStates[item.ID]})
	}
	return items, nil
}

func (f *fakeRepository) ListDueVocab(_ context.Context, userID string, now time.Time) ([]repository.VocabWithState, error) {
	items := make([]repository.VocabWithState, 0)
	for _, state := range f.reviewStates {
		if state.UserID != userID || state.NextDueAt.After(now) {
			continue
		}
		item := f.vocab[state.VocabItemID]
		if item.ArchivedAt != nil {
			continue
		}
		items = append(items, repository.VocabWithState{Item: item, State: state})
	}
	return items, nil
}

func (f *fakeRepository) GetReviewState(_ context.Context, vocabID string) (domain.ReviewState, bool, error) {
	state, ok := f.reviewStates[vocabID]
	return state, ok, nil
}

func (f *fakeRepository) RecordReview(_ context.Context, state domain.ReviewState, log domain.ReviewLog, job *domain.NotificationJob) error {
	f.reviewStates[state.VocabItemID] = state
	f.reviewLogs[log.ID] = log
	if job != nil {
		for _, existing := range f.notificationJobs {
			if existing.UserID == job.UserID && existing.VocabItemID == job.VocabItemID && existing.Status == "pending" {
				return nil
			}
		}
		f.notificationJobs[job.ID] = *job
	}
	return nil
}

func (f *fakeRepository) UpsertDeviceToken(_ context.Context, token domain.DeviceToken) (domain.DeviceToken, error) {
	for id, existing := range f.deviceTokens {
		if existing.UserID == token.UserID && existing.Token == token.Token {
			token.ID = id
			token.CreatedAt = existing.CreatedAt
		}
	}
	f.deviceTokens[token.ID] = token
	return token, nil
}

func (f *fakeRepository) ListNotificationJobs(_ context.Context, userID string) ([]domain.NotificationJob, error) {
	jobs := make([]domain.NotificationJob, 0)
	for _, job := range f.notificationJobs {
		if job.UserID == userID {
			jobs = append(jobs, job)
		}
	}
	return jobs, nil
}

func newTestApp() *App {
	return NewApp(newFakeRepository(), stubClock{now: time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)})
}

func TestReviewScheduling(t *testing.T) {
	app := newTestApp()
	link, err := app.RequestMagicLink("test@example.com", "http://localhost:8080")
	if err != nil {
		t.Fatalf("request link: %v", err)
	}
	auth, err := app.VerifyMagicLink(link["token"])
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	item, _, err := app.CreateVocab(auth.User.ID, CreateVocabInput{
		Term:    "serendipity",
		Meaning: "a happy accident",
	})
	if err != nil {
		t.Fatalf("create vocab: %v", err)
	}

	state, err := app.GradeReview(auth.User.ID, item.ID, domain.ReviewGradeGood)
	if err != nil {
		t.Fatalf("good review: %v", err)
	}
	if state.IntervalDays != 1 {
		t.Fatalf("expected interval 1, got %d", state.IntervalDays)
	}

	state, err = app.GradeReview(auth.User.ID, item.ID, domain.ReviewGradeEasy)
	if err != nil {
		t.Fatalf("easy review: %v", err)
	}
	if state.IntervalDays < 3 {
		t.Fatalf("expected easy interval >= 3, got %d", state.IntervalDays)
	}
}
