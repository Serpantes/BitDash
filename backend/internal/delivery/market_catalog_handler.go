package delivery

import (
	"net/http"
	"strings"

	"bitdash/backend/internal/repository"
)

func NewMarketCatalogHandler(catalog *repository.MarketCatalog) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, gzipped, err := catalog.Get()
		if err != nil {
			http.Error(w, "market catalog unavailable", http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Vary", "Accept-Encoding")
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			w.Header().Set("Content-Encoding", "gzip")
			w.Write(gzipped) //nolint:errcheck
			return
		}
		w.Write(body) //nolint:errcheck
	}
}
