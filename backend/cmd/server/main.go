package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"bitdash/backend/internal/delivery"
	"bitdash/backend/internal/repository"
	"bitdash/backend/internal/usecase"
)

func main() {
	addr := ":8080"
	external := os.Getenv("EXTERNAL_API_URL")
	bitjitaBaseURL := os.Getenv("BITJITA_BASE_URL")
	playerResponseFile := os.Getenv("PLAYER_RESPONSE_FILE")
	if external == "" {
		external = "https://api.example.com/data"
	}
	if bitjitaBaseURL == "" {
		bitjitaBaseURL = "https://bitjita.com"
	}
	if playerResponseFile == "" {
		playerResponseFile = "test.json"
	}

	repo := repository.NewExternalClient(external)
	handler := delivery.NewProxyHandler(repo)
	playerSource := repository.NewBitjitaPlayerSource(bitjitaBaseURL)
	playerStore := repository.NewPlayerFileStore(playerResponseFile)
	playerService := usecase.NewPlayerService(playerSource, playerStore)
	playerHandler := delivery.NewPlayerHandler(playerService)

	http.HandleFunc("/api/data", handler)
	http.HandleFunc("/api/health", delivery.NewHealthHandler())
	http.HandleFunc("/api/ping", delivery.NewPingHandler())
	http.HandleFunc("/api/players/", playerHandler)

	// Serve static frontend in production from ../frontend/dist if exists
	fs := http.FileServer(http.Dir("../frontend/dist"))
	http.Handle("/", fs)

	server := &http.Server{
		Addr:         addr,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("Starting backend on %s, proxy -> %s, bitjita -> %s, player-response-file -> %s", addr, external, bitjitaBaseURL, playerResponseFile)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
