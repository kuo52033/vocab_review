package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"vocabreview/backend/internal/clock"
	"vocabreview/backend/internal/httpapi"
	"vocabreview/backend/internal/repository/postgres"
	"vocabreview/backend/internal/service"
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

	app := service.NewApp(store, clock.RealClock{})
	server := httpapi.NewServer(app)

	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
