package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os"
	"time"

	"vocabreview/backend/internal/clock"
	"vocabreview/backend/internal/repository/postgres"
	"vocabreview/backend/internal/service/notifications"
)

func main() {
	once := flag.Bool("once", false, "process one notification batch and exit")
	interval := flag.Duration("interval", 10*time.Second, "poll interval for loop mode")
	batchSize := flag.Int("batch-size", 50, "maximum jobs to process per batch")
	flag.Parse()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, err := postgres.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer store.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	worker := notifications.NewWorker(
		store,
		notifications.DryRunSender{Logger: logger},
		clock.RealClock{},
		logger,
		notifications.Config{BatchSize: *batchSize},
	)

	if *once {
		if err := worker.RunOnce(ctx); err != nil {
			log.Fatalf("run notifications once: %v", err)
		}
		return
	}

	logger.Info("notification worker started", "interval", interval.String(), "batch_size", *batchSize, "mode", "dry_run")
	if err := worker.Run(ctx, *interval); err != nil {
		log.Fatalf("run notifications: %v", err)
	}
}
