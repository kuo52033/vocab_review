package main

import (
	"bytes"
	"context"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"vocabreview/backend/internal/clock"
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

	app := service.NewAppWithEnricher(store, clock.RealClock{}, newVocabEnricherFromEnv())
	logOutput := logFormatWriter{
		out:   os.Stdout,
		color: logColorEnabled(os.Getenv("LOG_COLOR")),
	}
	logger := slog.New(slog.NewTextHandler(logOutput, &slog.HandlerOptions{
		ReplaceAttr: replaceLogAttr,
	}))
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

func logColorEnabled(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
