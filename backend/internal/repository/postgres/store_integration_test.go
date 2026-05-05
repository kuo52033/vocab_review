package postgres

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"vocabreview/backend/internal/domain"
)

func TestStoreLifecycle(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for postgres integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resetDatabase(t, databaseURL)

	store, err := New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer store.Close()

	magicLink := domain.MagicLinkToken{
		Token:     "ml_test",
		Email:     "test@example.com",
		ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
	}
	if err := store.PutMagicLink(ctx, magicLink); err != nil {
		t.Fatalf("put magic link: %v", err)
	}

	user, session, err := store.ConsumeMagicLink(ctx, magicLink.Token, time.Now().UTC(), domain.User{
		ID:        "usr_test",
		CreatedAt: time.Now().UTC(),
	}, domain.Session{
		Token:     "sess_test",
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(30 * 24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("consume magic link: %v", err)
	}

	loadedSession, loadedUser, ok, err := store.GetSessionUser(ctx, session.Token)
	if err != nil {
		t.Fatalf("get session user: %v", err)
	}
	if !ok || loadedUser.ID != user.ID || loadedSession.Token != session.Token {
		t.Fatalf("unexpected session lookup: ok=%v user=%s session=%s", ok, loadedUser.ID, loadedSession.Token)
	}

	now := time.Now().UTC()
	item := domain.VocabItem{
		ID:              "voc_test",
		UserID:          user.ID,
		Term:            "serendipity",
		Kind:            domain.CardKindWord,
		Meaning:         "a happy accident",
		ExampleSentence: "Finding this test passing was serendipity.",
		SourceText:      "serendipity",
		SourceURL:       "https://example.com",
		Notes:           "integration",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	state := domain.ReviewState{
		VocabItemID:     item.ID,
		UserID:          user.ID,
		Status:          domain.ReviewStatusNew,
		EaseFactor:      2.5,
		IntervalDays:    0,
		RepetitionCount: 0,
		NextDueAt:       now,
	}
	job := &domain.NotificationJob{
		ID:          "job_test",
		UserID:      user.ID,
		VocabItemID: item.ID,
		ScheduledAt: now,
		Status:      "pending",
		Message:     "Review now",
	}
	if err := store.CreateVocab(ctx, item, state, job); err != nil {
		t.Fatalf("create vocab: %v", err)
	}

	listed, err := store.ListVocabByUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("list vocab: %v", err)
	}
	if len(listed) != 1 || listed[0].Item.ID != item.ID {
		t.Fatalf("unexpected vocab list: %+v", listed)
	}

	due, err := store.ListDueVocab(ctx, user.ID, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("list due: %v", err)
	}
	if len(due) != 1 || due[0].State.VocabItemID != item.ID {
		t.Fatalf("unexpected due list: %+v", due)
	}

	state.Status = domain.ReviewStatusReview
	state.IntervalDays = 1
	state.RepetitionCount = 1
	state.NextDueAt = now.Add(24 * time.Hour)
	reviewedAt := now.Add(5 * time.Minute)
	state.LastReviewedAt = &reviewedAt
	log := domain.ReviewLog{
		ID:          "rev_test",
		UserID:      user.ID,
		VocabItemID: item.ID,
		Grade:       domain.ReviewGradeGood,
		ReviewedAt:  reviewedAt,
	}
	if err := store.RecordReview(ctx, state, log, nil); err != nil {
		t.Fatalf("record review: %v", err)
	}

	loadedState, ok, err := store.GetReviewState(ctx, item.ID)
	if err != nil {
		t.Fatalf("get review state: %v", err)
	}
	if !ok || loadedState.IntervalDays != 1 {
		t.Fatalf("unexpected review state: ok=%v state=%+v", ok, loadedState)
	}

	history, err := store.ListReviewHistory(ctx, user.ID, 10)
	if err != nil {
		t.Fatalf("list review history: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected one review history entry, got %d", len(history))
	}
	if history[0].Log.ID != log.ID || history[0].Item.ID != item.ID || history[0].State.IntervalDays != 1 {
		t.Fatalf("unexpected review history: %+v", history)
	}

	device, err := store.UpsertDeviceToken(ctx, domain.DeviceToken{
		ID:        "dev_test",
		UserID:    user.ID,
		Platform:  "ios",
		Token:     "token-1",
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("upsert device: %v", err)
	}
	if device.ID == "" {
		t.Fatal("expected stored device id")
	}

	jobs, err := store.ListNotificationJobs(ctx, user.ID)
	if err != nil {
		t.Fatalf("list notification jobs: %v", err)
	}
	if len(jobs) != 1 || jobs[0].ID != job.ID {
		t.Fatalf("unexpected jobs: %+v", jobs)
	}
}

func resetDatabase(t *testing.T, databaseURL string) {
	t.Helper()

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("set dialect: %v", err)
	}

	migrationsDir := filepath.Join("..", "..", "..", "migrations")
	if err := goose.Reset(db, migrationsDir); err != nil {
		t.Fatalf("reset migrations: %v", err)
	}
	if err := goose.Up(db, migrationsDir); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
}
