package postgres

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/repository"
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

	listed, total, err := store.ListVocabByUser(ctx, user.ID, repository.ListVocabOptions{})
	if err != nil {
		t.Fatalf("list vocab: %v", err)
	}
	if len(listed) != 1 || total != 1 || listed[0].Item.ID != item.ID {
		t.Fatalf("unexpected vocab list: total=%d items=%+v", total, listed)
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

	history, historyTotal, err := store.ListReviewHistory(ctx, user.ID, repository.Pagination{Limit: 10})
	if err != nil {
		t.Fatalf("list review history: %v", err)
	}
	if len(history) != 1 || historyTotal != 1 {
		t.Fatalf("expected one review history entry, got total=%d items=%d", historyTotal, len(history))
	}
	if history[0].Log.ID != log.ID || history[0].Item.ID != item.ID || history[0].State.IntervalDays != 1 {
		t.Fatalf("unexpected review history: %+v", history)
	}

	stats, err := store.GetReviewStats(ctx, user.ID, reviewedAt)
	if err != nil {
		t.Fatalf("get review stats: %v", err)
	}
	if stats.ReviewedToday != 1 || stats.Reviewed7Days != 1 || stats.ActiveCards != 1 || stats.DueNow != 0 || stats.ArchivedCards != 0 {
		t.Fatalf("unexpected review stats: %+v", stats)
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

func TestArchiveVocabForUserScopesArchiveByOwner(t *testing.T) {
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

	now := time.Now().UTC()
	user := domain.User{
		ID:        "usr_owner",
		Email:     "owner@example.com",
		CreatedAt: now,
	}
	otherUser := domain.User{
		ID:        "usr_other",
		Email:     "other@example.com",
		CreatedAt: now,
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO users (id, email, created_at)
		VALUES ($1, $2, $3), ($4, $5, $6)
	`, user.ID, user.Email, user.CreatedAt, otherUser.ID, otherUser.Email, otherUser.CreatedAt); err != nil {
		t.Fatalf("insert users: %v", err)
	}

	item := domain.VocabItem{
		ID:              "voc_archive",
		UserID:          user.ID,
		Term:            "archive",
		Kind:            domain.CardKindWord,
		Meaning:         "store away",
		ExampleSentence: "Archive this card.",
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
	if err := store.CreateVocab(ctx, item, state, nil); err != nil {
		t.Fatalf("create vocab: %v", err)
	}

	archivedAt := now.Add(time.Minute)
	if _, err := store.ArchiveVocabForUser(ctx, otherUser.ID, item.ID, archivedAt); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("archive as other user: got %v want %v", err, repository.ErrNotFound)
	}

	archived, err := store.ArchiveVocabForUser(ctx, user.ID, item.ID, archivedAt)
	if err != nil {
		t.Fatalf("archive as owner: %v", err)
	}
	if archived.ArchivedAt == nil || !archived.ArchivedAt.Equal(archivedAt) {
		t.Fatalf("archived at: got %v want %s", archived.ArchivedAt, archivedAt)
	}
	if !archived.UpdatedAt.Equal(archivedAt) {
		t.Fatalf("updated at: got %s want %s", archived.UpdatedAt, archivedAt)
	}
}

func TestStorePersistsPartOfSpeech(t *testing.T) {
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

	now := time.Now().UTC()
	user := domain.User{ID: "usr_pos", Email: "pos@example.com", CreatedAt: now}
	if _, err := store.pool.Exec(ctx, `INSERT INTO users (id, email, created_at) VALUES ($1, $2, $3)`, user.ID, user.Email, user.CreatedAt); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	item := domain.VocabItem{
		ID:              "voc_pos",
		UserID:          user.ID,
		Term:            "serendipity",
		Kind:            domain.CardKindWord,
		Meaning:         "happy accident",
		ExampleSentence: "It was serendipity.",
		PartOfSpeech:    domain.PartOfSpeechNoun,
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
	if err := store.CreateVocab(ctx, item, state, nil); err != nil {
		t.Fatalf("create vocab: %v", err)
	}

	loaded, ok, err := store.GetVocab(ctx, item.ID)
	if err != nil {
		t.Fatalf("get vocab: %v", err)
	}
	if !ok {
		t.Fatal("expected vocab")
	}
	if loaded.PartOfSpeech != domain.PartOfSpeechNoun {
		t.Fatalf("part of speech: got %q want %q", loaded.PartOfSpeech, domain.PartOfSpeechNoun)
	}

	item.ID = "voc_invalid_pos"
	item.PartOfSpeech = domain.PartOfSpeech("invalid")
	err = store.CreateVocab(ctx, item, domain.ReviewState{
		VocabItemID:     item.ID,
		UserID:          user.ID,
		Status:          domain.ReviewStatusNew,
		EaseFactor:      2.5,
		IntervalDays:    0,
		RepetitionCount: 0,
		NextDueAt:       now,
	}, nil)
	if err == nil {
		t.Fatal("expected invalid part_of_speech constraint error")
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
