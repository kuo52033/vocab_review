package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"

	"vocabreview/backend/internal/clock"
	"vocabreview/backend/internal/repository/postgres"
	"vocabreview/backend/internal/service/audios"
)

func main() {
	once := flag.Bool("once", false, "process one audio batch and exit")
	interval := flag.Duration("interval", 10*time.Second, "poll interval for loop mode")
	batchSize := flag.Int("batch-size", 10, "maximum jobs to process per batch")
	flag.Parse()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	config, err := audioConfigFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, err := postgres.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer store.Close()

	awsCtx, awsCancel := context.WithTimeout(ctx, 5*time.Second)
	defer awsCancel()
	awsConfig, err := awsconfig.LoadDefaultConfig(awsCtx, awsconfig.WithRegion(config.region))
	if err != nil {
		log.Fatalf("load AWS config: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	worker := audios.NewWorker(
		store,
		audios.NewOpenAISpeechClient(config.baseURL, config.apiKey, &http.Client{Timeout: 60 * time.Second}),
		audios.NewS3Storage(awsConfig, config.bucket),
		clock.RealClock{},
		logger,
		audios.Config{BatchSize: *batchSize},
	)

	if *once {
		if err := worker.RunOnce(ctx); err != nil {
			log.Fatalf("run audio worker once: %v", err)
		}
		return
	}

	logger.Info("audio worker started", "interval", interval.String(), "batch_size", *batchSize)
	if err := worker.Run(ctx, *interval); err != nil {
		log.Fatalf("run audio worker: %v", err)
	}
}

type audioEnvConfig struct {
	baseURL string
	apiKey  string
	bucket  string
	region  string
}

func audioConfigFromEnv() (audioEnvConfig, error) {
	config := audioEnvConfig{
		baseURL: strings.TrimSpace(os.Getenv("TTS_OPENAI_BASE_URL")),
		apiKey:  strings.TrimSpace(os.Getenv("TTS_OPENAI_API_KEY")),
		bucket:  strings.TrimSpace(os.Getenv("TTS_S3_BUCKET")),
		region:  strings.TrimSpace(os.Getenv("TTS_S3_REGION")),
	}
	if config.baseURL == "" {
		config.baseURL = "https://api.openai.com/v1"
	}
	if config.apiKey == "" {
		return audioEnvConfig{}, errors.New("TTS_OPENAI_API_KEY is required")
	}
	if config.bucket == "" {
		return audioEnvConfig{}, errors.New("TTS_S3_BUCKET is required")
	}
	if config.region == "" {
		config.region = strings.TrimSpace(os.Getenv("AWS_REGION"))
	}
	if config.region == "" {
		return audioEnvConfig{}, errors.New("TTS_S3_REGION or AWS_REGION is required")
	}
	return config, nil
}
