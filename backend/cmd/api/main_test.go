package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestStatusColorWriterColorsByStatusFamily(t *testing.T) {
	tests := []struct {
		name   string
		status string
		color  string
	}{
		{name: "success", status: "200", color: "\033[32m"},
		{name: "redirect", status: "302", color: "\033[36m"},
		{name: "client error", status: "404", color: "\033[33m"},
		{name: "server error", status: "500", color: "\033[31m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			writer := logFormatWriter{out: &output, color: true}

			if _, err := writer.Write([]byte("msg=request status=" + tt.status + "\n")); err != nil {
				t.Fatalf("write: %v", err)
			}

			got := output.String()
			expected := "status=" + tt.color + tt.status + "\033[0m"
			if !strings.Contains(got, expected) {
				t.Fatalf("expected colored status %q, got %q", expected, got)
			}
			if strings.HasPrefix(got, tt.color) {
				t.Fatalf("expected only status value to be colored, got %q", got)
			}
		})
	}
}

func TestStatusColorWriterLeavesNonRequestLinesPlain(t *testing.T) {
	var output bytes.Buffer
	writer := logFormatWriter{out: &output, color: true}

	if _, err := writer.Write([]byte("listening on :8080\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	if got := output.String(); got != "listening on :8080\n" {
		t.Fatalf("expected plain line, got %q", got)
	}
}

func TestLogFormatWriterStripsTimeKey(t *testing.T) {
	var output bytes.Buffer
	writer := logFormatWriter{out: &output}

	if _, err := writer.Write([]byte("time=\"[2026-05-05T01:02:03.456]\" level=INFO msg=request status=200\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	if got := output.String(); got != "[2026-05-05T01:02:03.456] level=INFO msg=request status=200\n" {
		t.Fatalf("unexpected formatted log: %q", got)
	}
}

func TestTextLoggerOutputOmitsTimeKey(t *testing.T) {
	var output bytes.Buffer
	writer := logFormatWriter{out: &output}
	logger := slog.New(slog.NewTextHandler(writer, &slog.HandlerOptions{
		ReplaceAttr: replaceLogAttr,
	}))

	logger.Info("request", "status", 200)

	got := output.String()
	if strings.Contains(got, "time=") {
		t.Fatalf("expected log to omit time key, got %q", got)
	}
	if !strings.HasPrefix(got, "[") {
		t.Fatalf("expected bracketed timestamp prefix, got %q", got)
	}
}

func TestLogColorEnabled(t *testing.T) {
	for _, value := range []string{"1", "true", "TRUE", "yes", "on"} {
		if !logColorEnabled(value) {
			t.Fatalf("expected %q to enable color", value)
		}
	}
	for _, value := range []string{"", "0", "false", "off", "no"} {
		if logColorEnabled(value) {
			t.Fatalf("expected %q to disable color", value)
		}
	}
}

func TestReplaceLogAttrFormatsTimeWithoutZone(t *testing.T) {
	attr := replaceLogAttr(nil, slog.Time(slog.TimeKey, time.Date(2026, 5, 5, 1, 2, 3, 456000000, time.FixedZone("CST", 8*60*60))))

	if attr.Key != slog.TimeKey {
		t.Fatalf("expected time key, got %q", attr.Key)
	}
	if got := attr.Value.String(); got != "[2026-05-05T01:02:03.456]" {
		t.Fatalf("unexpected time format: %q", got)
	}
}
