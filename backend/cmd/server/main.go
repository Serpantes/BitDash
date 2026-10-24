package main

import (
	"context"
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
	bitjitaCacheDir := os.Getenv("BITJITA_CACHE_DIR")
	xpHistoryDir := os.Getenv("XP_HISTORY_DIR")
	dashboardFile := os.Getenv("DASHBOARD_FILE")
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
	if bitjitaCacheDir == "" {
		bitjitaCacheDir = "/var/lib/bitdash/bitjita-cache"
	}
	if xpHistoryDir == "" {
		xpHistoryDir = "/var/lib/bitdash/xp-history"
	}

	if dashboardFile == "" {
		dashboardFile = "/var/lib/bitdash/dashboard.json"
	}

	liveCacheTTL := 2 * time.Minute

	repo := repository.NewExternalClient(external)
	handler := delivery.NewProxyHandler(repo)
	playerSource := repository.NewBitjitaPlayerSource(bitjitaBaseURL)
	playerStore := repository.NewPlayerFileStore(playerResponseFile)
	playerService := usecase.NewPlayerService(playerSource, playerStore)
	playerHandler := delivery.NewPlayerHandler(playerService)

	liveBitjitaCache := cache.New(liveCacheTTL)
	bitjitaDataCache, err := repository.NewBitjitaCache(bitjitaBaseURL, bitjitaCacheDir, liveBitjitaCache)
	if err != nil {
		log.Fatalf("bitjita cache: %v", err)
	}

	xpHistory, err := repository.NewXPHistory(xpHistoryDir, "wss://live.bitjita.com", bitjitaDataCache)
	if err != nil {
		log.Fatalf("xp history: %v", err)
	}
	xpHistory.Start(context.Background())

	mapUpstreamCache := cache.New(3 * time.Minute)
	bitjitaProxyHandler := delivery.NewBitjitaProxyHandler(bitjitaDataCache)
	bitjitaIconHandler := delivery.NewBitjitaIconHandler(bitjitaDataCache)

	itemNameStore, err := repository.NewItemNameStore(itemNameFile)
	if err != nil {
		log.Fatalf("item name store: %v", err)
	}
	itemNamesHandler := delivery.NewItemNamesHandler(bitjitaDataCache, itemNameStore)
	dashboardStore, err := repository.NewDashboardStore(dashboardFile)
	if err != nil {
		log.Fatalf("dashboard store: %v", err)
	}
	marketCatalog := repository.NewMarketCatalog(bitjitaDataCache, 10*time.Minute)
	smithingOre := repository.NewSmithingOre(bitjitaDataCache, 5*time.Minute)
	go smithingOre.Warm()

	http.HandleFunc("/api/data", handler)
	http.HandleFunc("/api/health", delivery.NewHealthHandler())
	http.HandleFunc("/api/ping", delivery.NewPingHandler())
	http.HandleFunc("/api/players/", playerHandler)
	http.HandleFunc("/api/bitjita/", bitjitaProxyHandler)
	http.HandleFunc("/api/bitjita-icon", bitjitaIconHandler)
	http.HandleFunc("/api/item-names", itemNamesHandler)
	http.HandleFunc("/api/market-catalog", delivery.NewMarketCatalogHandler(marketCatalog))
	http.HandleFunc("/api/dashboard", delivery.NewDashboardHandler(dashboardStore))
	http.HandleFunc("/api/xp-history/", delivery.NewXPHistoryHandler(xpHistory))
	http.HandleFunc("/api/smithing-ore/", delivery.NewSmithingOreHandler(smithingOre))
	http.Handle("/map-embed", delivery.NewMapEmbedHandler())
	http.Handle("/map-embed/", delivery.NewMapEmbedHandler())
	http.Handle("/api/map-upstream/", delivery.NewMapUpstreamProxyHandler(mapUpstreamCache))
	http.HandleFunc("/api/live-ws/", delivery.NewLiveBitjitaWebSocketProxyHandler("/api/live-ws"))
	http.HandleFunc("/api/live-ws", delivery.NewLiveBitjitaWebSocketProxyHandler("/api/live-ws"))
	http.HandleFunc("/favicon.ico", delivery.NewFaviconHandler())

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

	log.Printf(
		"Starting backend on %s, proxy -> %s, bitjita -> %s, bitjita-cache -> %s, xp-history -> %s, dashboard -> %s, player-response-file -> %s, item-name-file -> %s",
		addr, external, bitjitaBaseURL, bitjitaCacheDir, xpHistoryDir, dashboardFile, playerResponseFile, itemNameFile,
	)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
