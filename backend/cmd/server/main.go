package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
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
	dashboardDir := os.Getenv("DASHBOARD_DIR")
	dashboardTemplateFile := os.Getenv("DASHBOARD_TEMPLATE_FILE")
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

	if dashboardDir == "" {
		dashboardDir = "/var/lib/bitdash/dashboards"
	}
	rateLimitRPS := envFloat("RATE_LIMIT_RPS", 10)
	rateLimitBurst := int(envFloat("RATE_LIMIT_BURST", 300))

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
	relayURL := os.Getenv("BITJITA_RELAY_URL")
	if relayURL == "" {
		relayURL = "https://relay.bitjita.com"
	}
	if relayURL != "off" {
		bitjitaDataCache.UseRelay(relayURL)
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
	dashboardStores, err := repository.NewDashboardStores(dashboardDir, dashboardTemplateFile)
	if err != nil {
		log.Fatalf("dashboard store: %v", err)
	}
	// The full market listing is ~10 MB; names and buy/sell flags drift slowly.
	marketCatalog := repository.NewMarketCatalog(bitjitaDataCache, 30*time.Minute)
	smithingOre := repository.NewSmithingOre(bitjitaDataCache, 5*time.Minute, 10*time.Minute)
	go smithingOre.Run()
	storageCapacity := repository.NewStorageCapacity(bitjitaDataCache, 24*time.Hour)

	http.HandleFunc("/api/data", handler)
	http.HandleFunc("/api/health", delivery.NewHealthHandler())
	http.HandleFunc("/api/ping", delivery.NewPingHandler())
	http.HandleFunc("/api/players/", playerHandler)
	http.HandleFunc("/api/bitjita/", bitjitaProxyHandler)
	http.HandleFunc("/api/bitjita-icon", bitjitaIconHandler)
	http.HandleFunc("/api/bitjita-stats", delivery.NewBitjitaStatsHandler(bitjitaDataCache))
	http.HandleFunc("/api/item-names", itemNamesHandler)
	http.HandleFunc("/api/market-catalog", delivery.NewMarketCatalogHandler(marketCatalog))
	http.HandleFunc("/api/inventory-slots", delivery.NewInventorySlotsHandler(bitjitaDataCache.InventorySlots()))
	http.HandleFunc("/api/storage-capacity", delivery.NewStorageCapacityHandler(storageCapacity))
	http.Handle("/api/dashboard", delivery.WithVisitor(delivery.NewDashboardHandler(dashboardStores)))
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
		Handler:      delivery.NewRateLimiter(rateLimitRPS, rateLimitBurst).Wrap(http.DefaultServeMux),
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 3 * time.Minute,
	}

	log.Printf(
		"Starting backend on %s, proxy -> %s, bitjita -> %s, bitjita-cache -> %s, xp-history -> %s, dashboards -> %s, player-response-file -> %s, item-name-file -> %s, rate limit %.0f/s burst %d",
		addr, external, bitjitaBaseURL, bitjitaCacheDir, xpHistoryDir, dashboardDir, playerResponseFile, itemNameFile, rateLimitRPS, rateLimitBurst,
	)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func envFloat(name string, fallback float64) float64 {
	v, err := strconv.ParseFloat(os.Getenv(name), 64)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}
