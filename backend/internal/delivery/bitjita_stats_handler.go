package delivery

import (
	"encoding/json"
	"net/http"

	"bitdash/backend/internal/repository"
)

// NewBitjitaStatsHandler reports how many requests reached BitJita, per path; ?reset=1 starts over.
func NewBitjitaStatsHandler(c *repository.BitjitaCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")
		json.NewEncoder(w).Encode(c.UpstreamStats(r.URL.Query().Get("reset") == "1")) //nolint:errcheck
	}
}
