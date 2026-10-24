package delivery

import (
	"encoding/json"
	"net/http"
	"strings"

	"bitdash/backend/internal/repository"
)

func NewSmithingOreHandler(ore *repository.SmithingOre) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		playerID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/smithing-ore/"), "/")
		if !isAllDigits(playerID) {
			http.Error(w, "player id must be numeric", http.StatusBadRequest)
			return
		}
		payload, err := ore.Get(playerID)
		if err != nil {
			http.Error(w, "smithing ore data unavailable", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")
		json.NewEncoder(w).Encode(payload) //nolint:errcheck
	}
}
