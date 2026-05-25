package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/service/enrichment"
)

type vocabRow struct {
	ID              string
	Term            string
	Meaning         string
	ExampleSentence string
	PartOfSpeech    domain.PartOfSpeech
}

func main() {
	apply := flag.Bool("apply", false, "write generated Chinese values to the database")
	limit := flag.Int("limit", 0, "maximum number of cards to process; 0 means no limit")
	batchSize := flag.Int("batch", 20, "number of cards to send to GPT per request; max 20")
	flag.Parse()

	if *batchSize < 1 || *batchSize > 20 {
		log.Fatalf("-batch must be between 1 and 20, got %d", *batchSize)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	baseURL := os.Getenv("VOCAB_ENRICHMENT_BASE_URL")
	apiKey := os.Getenv("VOCAB_ENRICHMENT_API_KEY")
	model := os.Getenv("VOCAB_ENRICHMENT_MODEL")
	if baseURL == "" || apiKey == "" || model == "" {
		log.Fatal("VOCAB_ENRICHMENT_BASE_URL, VOCAB_ENRICHMENT_API_KEY, and VOCAB_ENRICHMENT_MODEL are required")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer pool.Close()

	provider := enrichment.NewOpenAIProvider(baseURL, apiKey, model, &http.Client{Timeout: 60 * time.Second})
	enricher := enrichment.New(provider, *batchSize)

	cards, err := loadCards(ctx, pool, *limit)
	if err != nil {
		log.Fatalf("load cards: %v", err)
	}
	if len(cards) == 0 {
		log.Print("no active cards need Chinese backfill")
		return
	}

	mode := "dry-run"
	if *apply {
		mode = "apply"
	}
	log.Printf("starting Chinese backfill mode=%s cards=%d batch=%d", mode, len(cards), *batchSize)

	updated := 0
	skipped := 0
	for start := 0; start < len(cards); start += *batchSize {
		end := min(start+*batchSize, len(cards))
		batch := cards[start:end]
		suggestions, err := enricher.Autocomplete(ctx, enrichmentItems(batch))
		if err != nil {
			log.Fatalf("autocomplete batch %d-%d: %v", start+1, end, err)
		}
		for i, card := range batch {
			if i >= len(suggestions) {
				log.Fatalf("autocomplete returned too few suggestions for batch %d-%d", start+1, end)
			}
			chinese := strings.TrimSpace(suggestions[i].Chinese)
			if chinese == "" {
				skipped++
				log.Printf("skip id=%s term=%q reason=empty_chinese", card.ID, card.Term)
				continue
			}
			if !*apply {
				log.Printf("dry-run id=%s term=%q chinese=%q", card.ID, card.Term, chinese)
				updated++
				continue
			}
			if err := updateChinese(ctx, pool, card.ID, chinese); err != nil {
				log.Fatalf("update id=%s term=%q: %v", card.ID, card.Term, err)
			}
			updated++
			log.Printf("updated id=%s term=%q chinese=%q", card.ID, card.Term, chinese)
		}
	}

	log.Printf("Chinese backfill complete mode=%s candidates=%d %s=%d skipped=%d", mode, len(cards), resultLabel(*apply), updated, skipped)
	if !*apply {
		log.Print("dry-run only; rerun with -apply to write these values")
	}
}

func loadCards(ctx context.Context, pool *pgxpool.Pool, limit int) ([]vocabRow, error) {
	query := `
		SELECT id, term, meaning, example_sentence, part_of_speech
		FROM vocab_items
		WHERE archived_at IS NULL
		  AND btrim(chinese) = ''
		ORDER BY created_at ASC
	`
	args := []any{}
	if limit > 0 {
		query += " LIMIT $1"
		args = append(args, limit)
	}

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cards := []vocabRow{}
	for rows.Next() {
		var card vocabRow
		if err := rows.Scan(&card.ID, &card.Term, &card.Meaning, &card.ExampleSentence, &card.PartOfSpeech); err != nil {
			return nil, err
		}
		cards = append(cards, card)
	}
	return cards, rows.Err()
}

func enrichmentItems(cards []vocabRow) []enrichment.Item {
	items := make([]enrichment.Item, 0, len(cards))
	for _, card := range cards {
		items = append(items, enrichment.Item{
			Term:            card.Term,
			Meaning:         card.Meaning,
			ExampleSentence: card.ExampleSentence,
			PartOfSpeech:    card.PartOfSpeech,
		})
	}
	return items
}

func updateChinese(ctx context.Context, pool *pgxpool.Pool, id string, chinese string) error {
	tag, err := pool.Exec(ctx, `
		UPDATE vocab_items
		SET chinese = $2,
		    updated_at = now()
		WHERE id = $1
		  AND archived_at IS NULL
		  AND btrim(chinese) = ''
	`, id, chinese)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("expected to update 1 row, updated %d", tag.RowsAffected())
	}
	return nil
}

func resultLabel(apply bool) string {
	if apply {
		return "updated"
	}
	return "would_update"
}
