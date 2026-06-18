package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"

	"vocabreview/backend/internal/clock"
	"vocabreview/backend/internal/email"
	"vocabreview/backend/internal/httpapi"
	"vocabreview/backend/internal/repository/postgres"
	"vocabreview/backend/internal/service"
	"vocabreview/backend/internal/service/audios"
	"vocabreview/backend/internal/service/enrichment"
)

func main() {
	addr := ":8080"
	if value := os.Getenv("ADDR"); value != "" {
		addr = value
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	store, err := postgres.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer store.Close()

	authConfig, err := newAuthConfigFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	awsCtx, awsCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer awsCancel()

	logger := newLogger(os.Stdout, os.Getenv("LOG_COLOR"))

	magicLinkSender, err := newMagicLinkSender(awsCtx, authConfig.Environment, logger)
	if err != nil {
		log.Fatal(err)
	}
	audioConfig := newVocabAudioConfigFromEnv()
	audioURLSigner, err := newVocabAudioURLSignerFromEnv(awsCtx)
	if err != nil {
		log.Fatal(err)
	}
	app := service.NewAppWithVocabAudioConfigAndSigner(
		store, clock.RealClock{},
		newVocabEnricherFromEnv(),
		authConfig,
		magicLinkSender,
		audioConfig,
		audioURLSigner,
	)
	server := httpapi.NewServer(app, logger)

	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

func newVocabEnricherFromEnv() service.VocabEnricher {
	baseURL := os.Getenv("VOCAB_ENRICHMENT_BASE_URL")
	apiKey := os.Getenv("VOCAB_ENRICHMENT_API_KEY")
	model := os.Getenv("VOCAB_ENRICHMENT_MODEL")
	if baseURL == "" || apiKey == "" || model == "" {
		return nil
	}
	provider := enrichment.NewOpenAIProvider(baseURL, apiKey, model, &http.Client{Timeout: 15 * time.Second})
	return enrichment.New(provider, 20)
}

func newVocabAudioConfigFromEnv() service.VocabAudioConfig {
	apiKey := strings.TrimSpace(os.Getenv("TTS_OPENAI_API_KEY"))
	bucket := strings.TrimSpace(os.Getenv("TTS_S3_BUCKET"))
	region := strings.TrimSpace(os.Getenv("TTS_S3_REGION"))
	if region == "" {
		region = strings.TrimSpace(os.Getenv("AWS_REGION"))
	}
	if apiKey == "" || bucket == "" || region == "" {
		return service.VocabAudioConfig{}
	}
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
	return service.VocabAudioConfig{
		Enabled:       true,
		Provider:      "openai",
		Model:         model,
		Voice:         voice,
		Speed:         1,
		OutputFormat:  outputFormat,
		PublicBaseURL: os.Getenv("TTS_AUDIO_PUBLIC_BASE_URL"),
	}
}

func newVocabAudioURLSignerFromEnv(ctx context.Context) (service.VocabAudioURLSigner, error) {
	bucket := strings.TrimSpace(os.Getenv("TTS_S3_BUCKET"))
	region := strings.TrimSpace(os.Getenv("TTS_S3_REGION"))
	if region == "" {
		region = strings.TrimSpace(os.Getenv("AWS_REGION"))
	}
	if bucket == "" || region == "" {
		return nil, nil
	}
	awsConfig, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load AWS config for audio presigner: %w", err)
	}
	return audios.NewS3Presigner(awsConfig, bucket, 5*time.Minute), nil
}

func newAuthConfigFromEnv() (service.AuthConfig, error) {
	environment := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))

	if environment == "" {
		environment = "production"
	}
	config := service.AuthConfig{
		Environment:      environment,
		TokenHashSecret:  os.Getenv("TOKEN_HASH_SECRET"),
		PublicWebBaseURL: os.Getenv("PUBLIC_WEB_BASE_URL"),
		DebugEmails:      splitCSV(os.Getenv("MAGIC_LINK_DEBUG_EMAILS")),
	}
	if environment == "development" {
		return config, nil
	}

	if config.TokenHashSecret == "" {
		return service.AuthConfig{}, errors.New("TOKEN_HASH_SECRET is required outside development")
	}
	if strings.TrimSpace(config.PublicWebBaseURL) == "" {
		return service.AuthConfig{}, errors.New("PUBLIC_WEB_BASE_URL is required outside development")
	}
	if strings.ToLower(strings.TrimSpace(os.Getenv("MAIL_PROVIDER"))) != "ses" {
		return service.AuthConfig{}, errors.New("MAIL_PROVIDER=ses is required outside development")
	}
	if strings.TrimSpace(os.Getenv("MAIL_FROM_EMAIL")) == "" {
		return service.AuthConfig{}, errors.New("MAIL_FROM_EMAIL is required outside development")
	}
	return config, nil
}

func newMagicLinkSender(ctx context.Context, environment string, logger *slog.Logger) (service.MagicLinkSender, error) {
	if environment == "development" {
		return nil, nil
	}
	awsConfig, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}
	if strings.TrimSpace(awsConfig.Region) == "" {
		return nil, errors.New("AWS_REGION is required outside development")
	}
	fromName := os.Getenv("MAIL_FROM_NAME")
	if strings.TrimSpace(fromName) == "" {
		fromName = "Vocab Review"
	}
	return loggingMagicLinkSender{sender: email.NewSESSender(awsConfig, os.Getenv("MAIL_FROM_EMAIL"), fromName), logger: logger}, nil
}

type loggingMagicLinkSender struct {
	sender service.MagicLinkSender
	logger *slog.Logger
}

func (s loggingMagicLinkSender) SendMagicLink(ctx context.Context, email, verificationURL, token string, expiresAt time.Time) error {
	err := s.sender.SendMagicLink(ctx, email, verificationURL, token, expiresAt)
	if err != nil {
		s.logger.Error("magic link email send failed", "email", email, "error", err)
	}
	return err
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func logColorEnabled(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
