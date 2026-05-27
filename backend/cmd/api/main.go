package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"

	"vocabreview/backend/internal/clock"
	"vocabreview/backend/internal/email"
	"vocabreview/backend/internal/httpapi"
	"vocabreview/backend/internal/repository/postgres"
	"vocabreview/backend/internal/service"
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
	logOutput := logFormatWriter{
		out:   os.Stdout,
		color: logColorEnabled(os.Getenv("LOG_COLOR")),
	}
	logger := slog.New(slog.NewTextHandler(logOutput, &slog.HandlerOptions{
		ReplaceAttr: replaceLogAttr,
	}))
	awsCtx, awsCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer awsCancel()
	magicLinkSender, err := newMagicLinkSender(awsCtx, authConfig.Environment, logger)
	if err != nil {
		log.Fatal(err)
	}
	app := service.NewAppWithConfig(store, clock.RealClock{}, newVocabEnricherFromEnv(), authConfig, magicLinkSender)
	server := httpapi.NewServer(app, logger)

	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

type logFormatWriter struct {
	out   io.Writer
	color bool
}

func (w logFormatWriter) Write(data []byte) (int, error) {
	lines := bytes.SplitAfter(data, []byte("\n"))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		formatted := stripTimeKey(line)
		if w.color {
			if colored, ok := colorStatusValue(formatted); ok {
				formatted = colored
			}
		}
		if _, err := w.out.Write(formatted); err != nil {
			return 0, err
		}
	}
	return len(data), nil
}

func stripTimeKey(line []byte) []byte {
	const key = "time="
	if !bytes.HasPrefix(line, []byte(key)) {
		return line
	}

	valueStart := len(key)
	if valueStart >= len(line) {
		return line
	}

	quoted := line[valueStart] == '"'
	if quoted {
		valueStart++
	}
	valueEnd := valueStart
	for valueEnd < len(line) {
		if quoted && line[valueEnd] == '"' {
			break
		}
		if !quoted && (line[valueEnd] == ' ' || line[valueEnd] == '\n' || line[valueEnd] == '\r' || line[valueEnd] == '\t') {
			break
		}
		valueEnd++
	}
	if valueEnd >= len(line) {
		return line
	}

	nextStart := valueEnd
	if quoted {
		nextStart++
	}
	if nextStart < len(line) && line[nextStart] == ' ' {
		nextStart++
	}

	formatted := make([]byte, 0, len(line)-len(key)-2)
	formatted = append(formatted, line[valueStart:valueEnd]...)
	if nextStart < len(line) {
		formatted = append(formatted, ' ')
		formatted = append(formatted, line[nextStart:]...)
	}
	return formatted
}

func colorStatusValue(line []byte) ([]byte, bool) {
	const key = "status="
	index := bytes.Index(line, []byte(key))
	if index == -1 {
		return nil, false
	}
	valueStart := index + len(key)
	valueEnd := valueStart
	for valueEnd < len(line) && line[valueEnd] >= '0' && line[valueEnd] <= '9' {
		valueEnd++
	}
	if valueStart == valueEnd {
		return nil, false
	}

	status, ok := logStatusValue(string(line[valueStart:valueEnd]))
	if !ok {
		return nil, false
	}

	color := statusColor(status)
	if color == "" {
		return nil, false
	}

	colored := make([]byte, 0, len(line)+len(color)+len("\033[0m"))
	colored = append(colored, line[:valueStart]...)
	colored = append(colored, color...)
	colored = append(colored, line[valueStart:valueEnd]...)
	colored = append(colored, "\033[0m"...)
	colored = append(colored, line[valueEnd:]...)
	return colored, true
}

func statusColor(status int) string {
	switch {
	case status >= 200 && status < 300:
		return "\033[32m"
	case status >= 300 && status < 400:
		return "\033[36m"
	case status >= 400 && status < 500:
		return "\033[33m"
	case status >= 500:
		return "\033[31m"
	default:
		return ""
	}
}

func logStatusValue(value string) (int, bool) {
	status, err := strconv.Atoi(value)
	return status, err == nil
}

func replaceLogAttr(_ []string, attr slog.Attr) slog.Attr {
	if attr.Key == slog.TimeKey {
		return slog.String(slog.TimeKey, "["+attr.Value.Time().Format("2006-01-02T15:04:05.000")+"]")
	}
	return attr
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

func (s loggingMagicLinkSender) SendMagicLink(ctx context.Context, email, verificationURL string, expiresAt time.Time) error {
	err := s.sender.SendMagicLink(ctx, email, verificationURL, expiresAt)
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
