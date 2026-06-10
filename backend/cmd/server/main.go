package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"bitdash/backend/internal/cache"
	"bitdash/backend/internal/delivery"
	"bitdash/backend/internal/repository"
	"bitdash/backend/internal/usecase"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}
	addr := ":" + port
	external := os.Getenv("EXTERNAL_API_URL")
	bitjitaBaseURL := os.Getenv("BITJITA_BASE_URL")
	playerResponseFile := os.Getenv("PLAYER_RESPONSE_FILE")
	itemNameFile := os.Getenv("ITEM_NAME_FILE")
	if external == "" {
		external = "https://api.example.com/data"
	}
	if bitjitaBaseURL == "" {
		bitjitaBaseURL = "https://bitjita.com"
	}
	if playerResponseFile == "" {
		playerResponseFile = "test.json"
	}
	if itemNameFile == "" {
		itemNameFile = "data/item-names.json"
	}

	cacheTTL := 30 * time.Second
	iconCacheTTL := 24 * time.Hour

	repo := repository.NewExternalClient(external)
	handler := delivery.NewProxyHandler(repo)
	playerSource := repository.NewBitjitaPlayerSource(bitjitaBaseURL)
	playerStore := repository.NewPlayerFileStore(playerResponseFile)
	playerService := usecase.NewPlayerService(playerSource, playerStore)
	playerHandler := delivery.NewPlayerHandler(playerService)
	bitjitaCache := cache.New(cacheTTL)
	bitjitaProxyHandler := delivery.NewBitjitaProxyHandler(bitjitaBaseURL, bitjitaCache)
	bitjitaIconCache := cache.New(iconCacheTTL)
	bitjitaIconHandler := delivery.NewBitjitaIconHandler(bitjitaBaseURL, bitjitaIconCache)

	itemNameStore, err := repository.NewItemNameStore(itemNameFile)
	if err != nil {
		log.Fatalf("item name store: %v", err)
	}
	itemNamesHandler := delivery.NewItemNamesHandler(bitjitaBaseURL, itemNameStore)

	http.HandleFunc("/api/data", handler)
	http.HandleFunc("/api/health", delivery.NewHealthHandler())
	http.HandleFunc("/api/ping", delivery.NewPingHandler())
	http.HandleFunc("/api/players/", playerHandler)
	http.HandleFunc("/api/bitjita/", bitjitaProxyHandler)
	http.HandleFunc("/api/bitjita-icon", bitjitaIconHandler)
	http.HandleFunc("/api/item-names", itemNamesHandler)

	frontendDist := os.Getenv("FRONTEND_DIST")
	if frontendDist == "" {
		frontendDist = "../frontend/dist"
	}
	fs := http.FileServer(http.Dir(frontendDist))
	http.Handle("/", fs)

	server := &http.Server{
		Addr:         addr,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 3 * time.Minute,
	}

	log.Printf("Starting backend on %s, proxy -> %s, bitjita -> %s, player-response-file -> %s, item-name-file -> %s", addr, external, bitjitaBaseURL, playerResponseFile, itemNameFile)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
