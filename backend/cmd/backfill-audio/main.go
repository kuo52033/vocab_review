package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"log"
	"os"
	"strings"
	"time"

	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/repository/postgres"
	"vocabreview/backend/internal/service/audios"
)

func main() {
	limit := flag.Int("limit", 0, "maximum number of missing-audio cards to inspect; 0 means no limit")
	dryRun := flag.Bool("dry-run", false, "show what would be attached or enqueued without writing")
	flag.Parse()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	config := audioGenerationConfigFromEnv()

	ctx := context.Background()
	store, err := postgres.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer store.Close()

	items, err := store.ListVocabItemsMissingAudio(ctx, *limit)
	if err != nil {
		log.Fatalf("list missing audio cards: %v", err)
	}
	if len(items) == 0 {
		log.Print("no active cards need audio backfill")
		return
	}

	now := time.Now().UTC()
	mode := "apply"
	if *dryRun {
		mode = "dry-run"
	}
	log.Printf("starting audio backfill mode=%s cards=%d provider=%s model=%s voice=%s format=%s", mode, len(items), config.Provider, config.Model, config.Voice, config.OutputFormat)

	var attached, enqueued, skipped int
	for _, item := range items {
		inputText := audios.NormalizeInput(item.Term)
		if inputText == "" {
			skipped++
			log.Printf("skip id=%s reason=empty_term", item.ID)
			continue
		}
		inputHash := audios.InputHash(config, inputText)
		audio, ok, err := store.GetReadyVocabAudio(ctx, config.Provider, config.Model, config.Voice, config.Speed, config.OutputFormat, inputHash)
		if err != nil {
			log.Fatalf("lookup ready audio id=%s term=%q: %v", item.ID, item.Term, err)
		}
		if ok {
			if *dryRun {
				attached++
				log.Printf("dry-run attach id=%s term=%q audio_id=%s", item.ID, item.Term, audio.ID)
				continue
			}
			updated, err := store.AttachReadyVocabAudio(ctx, item.ID, audio.ID, now)
			if err != nil {
				log.Fatalf("attach audio id=%s term=%q: %v", item.ID, item.Term, err)
			}
			if updated {
				attached++
				log.Printf("attached id=%s term=%q audio_id=%s", item.ID, item.Term, audio.ID)
			} else {
				skipped++
				log.Printf("skip id=%s term=%q reason=changed", item.ID, item.Term)
			}
			continue
		}

		job := audioJob(item, config, inputText, inputHash, now)
		if *dryRun {
			enqueued++
			log.Printf("dry-run enqueue id=%s term=%q input_hash=%s", item.ID, item.Term, inputHash)
			continue
		}
		created, err := store.EnqueueVocabAudioJob(ctx, job)
		if err != nil {
			log.Fatalf("enqueue audio job id=%s term=%q: %v", item.ID, item.Term, err)
		}
		if created {
			enqueued++
			log.Printf("enqueued id=%s term=%q job_id=%s input_hash=%s", item.ID, item.Term, job.ID, inputHash)
		} else {
			skipped++
			log.Printf("skip id=%s term=%q reason=changed", item.ID, item.Term)
		}
	}
	log.Printf("audio backfill complete mode=%s candidates=%d attached=%d enqueued=%d skipped=%d", mode, len(items), attached, enqueued, skipped)
}

func audioGenerationConfigFromEnv() audios.GenerationConfig {
	model := strings.TrimSpace(os.Getenv("TTS_OPENAI_MODEL"))
	if model == "" {
		model = "gpt-4o-mini-tts"
	}
	voice := strings.TrimSpace(os.Getenv("TTS_OPENAI_VOICE"))
	if voice == "" {
		voice = "alloy"
	}
	outputFormat := strings.TrimSpace(os.Getenv("TTS_OUTPUT_FORMAT"))
	if outputFormat == "" {
		outputFormat = "mp3"
	}
	return audios.GenerationConfig{
		Provider:     "openai",
		Model:        model,
		Voice:        voice,
		Speed:        1,
		OutputFormat: outputFormat,
	}.Normalized()
}

func audioJob(item domain.VocabItem, config audios.GenerationConfig, inputText, inputHash string, now time.Time) domain.VocabAudioJob {
	return domain.VocabAudioJob{
		ID:            newID("audjob"),
		VocabItemID:   item.ID,
		Provider:      config.Provider,
		Model:         config.Model,
		Voice:         config.Voice,
		Speed:         config.Speed,
		OutputFormat:  config.OutputFormat,
		InputText:     inputText,
		InputHash:     inputHash,
		Status:        "pending",
		MaxAttempts:   3,
		NextAttemptAt: now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func newID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(buf)
}
