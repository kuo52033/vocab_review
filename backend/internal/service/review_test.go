package service

import (
	"path/filepath"
	"testing"
	"time"

	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/repository"
)

type stubClock struct {
	now time.Time
}

func (s stubClock) Now() time.Time { return s.now }

func newTestApp(t *testing.T) *App {
	t.Helper()
	store, err := repository.NewStore(filepath.Join(t.TempDir(), "store.json"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	return NewApp(store, stubClock{now: time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)})
}

func TestReviewScheduling(t *testing.T) {
	app := newTestApp(t)
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
