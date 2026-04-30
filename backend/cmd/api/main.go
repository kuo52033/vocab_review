package main

import (
	"log"
	"net/http"
	"os"

	"vocabreview/backend/internal/clock"
	"vocabreview/backend/internal/httpapi"
	"vocabreview/backend/internal/repository"
	"vocabreview/backend/internal/service"
)

func main() {
	storePath := "data/dev-store.json"
	if value := os.Getenv("STORE_PATH"); value != "" {
		storePath = value
	}
	addr := ":8080"
	if value := os.Getenv("ADDR"); value != "" {
		addr = value
	}

	store, err := repository.NewStore(storePath)
	if err != nil {
		log.Fatalf("load store: %v", err)
	}

	app := service.NewApp(store, clock.RealClock{})
	server := httpapi.NewServer(app)

	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
