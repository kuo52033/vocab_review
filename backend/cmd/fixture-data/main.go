package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	fixtureUserID    = "usr_fixture"
	fixtureEmail     = "tim31422@gmail.com"
	fixtureToken     = "fixture-dev-token"
	devTokenSecret   = "development-token-hash-secret"
	sessionIDMonths  = 6
	defaultDBTimeout = 20 * time.Second
)

type fixtureCard struct {
	ID              string
	Term            string
	Meaning         string
	Chinese         string
	ExampleSentence string
	PartOfSpeech    string
	Notes           string
	Status          string
	IntervalDays    int
	RepetitionCount int
	NextDueOffset   time.Duration
}

var fixtureCards = []fixtureCard{
	{
		ID:              "voc_fixture_embassy",
		Term:            "embassy",
		Meaning:         "an official office of one country in another country",
		Chinese:         "大使館",
		ExampleSentence: "The embassy helped renew her passport.",
		PartOfSpeech:    "noun",
		Status:          "new",
		NextDueOffset:   -10 * time.Minute,
	},
	{
		ID:              "voc_fixture_estate_agent",
		Term:            "estate agent",
		Meaning:         "a person whose job is selling or renting homes",
		Chinese:         "房地產仲介",
		ExampleSentence: "The estate agent showed us three apartments.",
		PartOfSpeech:    "noun",
		Status:          "new",
		NextDueOffset:   -8 * time.Minute,
	},
	{
		ID:              "voc_fixture_prior_to",
		Term:            "prior to",
		Meaning:         "before a particular time or event",
		Chinese:         "在...之前",
		ExampleSentence: "Please arrive prior to the meeting.",
		PartOfSpeech:    "phrase",
		Status:          "new",
		NextDueOffset:   -6 * time.Minute,
	},
	{
		ID:              "voc_fixture_meticulous",
		Term:            "meticulous",
		Meaning:         "very careful and paying attention to detail",
		Chinese:         "一絲不苟的",
		ExampleSentence: "She kept meticulous notes during the interview.",
		PartOfSpeech:    "adjective",
		Status:          "learning",
		IntervalDays:    1,
		RepetitionCount: 1,
		NextDueOffset:   2 * time.Hour,
	},
	{
		ID:              "voc_fixture_serendipity",
		Term:            "serendipity",
		Meaning:         "finding something good by chance",
		Chinese:         "意外收穫",
		ExampleSentence: "Finding that cafe was pure serendipity.",
		PartOfSpeech:    "noun",
		Status:          "review",
		IntervalDays:    4,
		RepetitionCount: 3,
		NextDueOffset:   48 * time.Hour,
	},
}

func main() {
	allowProduction := flag.Bool("allow-production", false, "allow seeding when APP_ENV=production")
	flag.Parse()

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	appEnv := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	if appEnv == "" {
		appEnv = "development"
	}
	if appEnv == "production" && !*allowProduction {
		log.Fatal("refusing to seed fixtures with APP_ENV=production; pass -allow-production if you really intend this")
	}
	tokenSecret := os.Getenv("TOKEN_HASH_SECRET")
	if tokenSecret == "" && appEnv == "development" {
		tokenSecret = devTokenSecret
	}
	if tokenSecret == "" {
		log.Fatal("TOKEN_HASH_SECRET is required outside development")
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultDBTimeout)
	defer cancel()

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		log.Fatalf("parse database url: %v", err)
	}
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer pool.Close()

	now := time.Now().UTC().Truncate(time.Second)
	if err := seed(ctx, pool, tokenSecret, now); err != nil {
		log.Fatalf("seed fixtures: %v", err)
	}

	fmt.Printf("seeded fixture user: %s (%s)\n", fixtureEmail, fixtureUserID)
	fmt.Printf("session token: %s\n", fixtureToken)
	fmt.Printf("cards: %d\n", len(fixtureCards))
}

func seed(ctx context.Context, pool *pgxpool.Pool, tokenSecret string, now time.Time) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, `DELETE FROM magic_links WHERE email = $1`, fixtureEmail); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM users WHERE id = $1 OR email = $2`, fixtureUserID, fixtureEmail); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO users (id, email, created_at)
		VALUES ($1, $2, $3)
	`, fixtureUserID, fixtureEmail, now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO sessions (token_hash, user_id, created_at, expires_at)
		VALUES ($1, $2, $3, $4)
	`, hashToken(fixtureToken, tokenSecret), fixtureUserID, now, now.AddDate(0, sessionIDMonths, 0)); err != nil {
		return err
	}

	for index, card := range fixtureCards {
		createdAt := now.Add(-time.Duration(len(fixtureCards)-index) * time.Minute)
		if err := insertCard(ctx, tx, card, createdAt, now); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func insertCard(ctx context.Context, tx pgx.Tx, card fixtureCard, createdAt, now time.Time) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO vocab_items (
			id, user_id, term, meaning, chinese, example_sentence, part_of_speech,
			source_text, source_url, notes, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $3, '', $8, $9, $10)
	`, card.ID, fixtureUserID, card.Term, card.Meaning, card.Chinese, card.ExampleSentence, card.PartOfSpeech, card.Notes, createdAt, now); err != nil {
		return err
	}

	easeFactor := 2.5
	nextDueAt := now.Add(card.NextDueOffset)
	if _, err := tx.Exec(ctx, `
		INSERT INTO review_states (
			vocab_item_id, user_id, status, ease_factor, interval_days, repetition_count,
			last_reviewed_at, next_due_at, consecutive_again
		)
		VALUES ($1, $2, $3, $4, $5, $6, NULL, $7, 0)
	`, card.ID, fixtureUserID, card.Status, easeFactor, card.IntervalDays, card.RepetitionCount, nextDueAt); err != nil {
		return err
	}
	return nil
}

func hashToken(token, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(mac.Sum(nil))
}
