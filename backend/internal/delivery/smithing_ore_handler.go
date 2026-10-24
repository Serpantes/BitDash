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
		rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/smithing-ore/"), "/")
		playerID, view, _ := strings.Cut(rest, "/")
		if !isAllDigits(playerID) || (view != "" && view != "stock") {
			http.Error(w, "expected /api/smithing-ore/{playerId}[/stock]", http.StatusBadRequest)
			return
		}
		var payload interface{}
		var err error
		if view == "stock" {
			payload, err = ore.Stock(playerID)
		} else {
			payload, err = ore.Get(playerID)
		}
		if err != nil {
			http.Error(w, "smithing ore data unavailable", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")
		json.NewEncoder(w).Encode(payload) //nolint:errcheck
	}
}
