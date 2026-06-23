package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"log/slog"
	"net"
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
	wakeAddr := flag.String("wake-addr", "", "optional internal HTTP address for wake requests")
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
	if strings.TrimSpace(*wakeAddr) == "" {
		if err := worker.Run(ctx, *interval); err != nil {
			log.Fatalf("run audio worker: %v", err)
		}
		return
	}

	wakeToken := strings.TrimSpace(os.Getenv("AUDIO_WORKER_WAKE_TOKEN"))
	if wakeToken == "" {
		log.Fatal("AUDIO_WORKER_WAKE_TOKEN is required when -wake-addr is configured")
	}
	wake := make(chan struct{}, 1)
	if err := startWakeServer(ctx, *wakeAddr, wakeToken, wake, logger); err != nil {
		log.Fatalf("start wake server: %v", err)
	}
	if err := runWakeLoop(ctx, worker, *interval, wake, logger); err != nil {
		log.Fatalf("run audio worker: %v", err)
	}
}

func runWakeLoop(ctx context.Context, worker *audios.Worker, interval time.Duration, wake <-chan struct{}, logger *slog.Logger) error {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	if err := worker.Drain(ctx); err != nil {
		return err
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-wake:
			logger.Info("audio worker wake received")
			if err := worker.Drain(ctx); err != nil {
				logger.Error("audio worker wake drain failed", "error", err)
			}
		case <-ticker.C:
			if err := worker.Drain(ctx); err != nil {
				logger.Error("audio worker fallback drain failed", "error", err)
			}
		}
	}
}

func startWakeServer(ctx context.Context, addr, token string, wake chan<- struct{}, logger *slog.Logger) error {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /wake", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Audio-Worker-Wake-Token") != token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		select {
		case wake <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusAccepted)
	})
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 2 * time.Second}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	go func() {
		logger.Info("audio worker wake server started", "addr", addr)
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("audio worker wake server failed", "error", err)
		}
	}()
	return nil
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
