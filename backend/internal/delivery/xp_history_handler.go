package delivery

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"bitdash/backend/internal/repository"
)

// NewXPHistoryHandler serves GET /api/xp-history/{playerId}?since=<unix>.
// Reading a player's history also (re)starts recording it.
func NewXPHistoryHandler(h *repository.XPHistory) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		playerID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/xp-history/"), "/")
		if !isAllDigits(playerID) {
			http.Error(w, "player id must be numeric", http.StatusBadRequest)
			return
		}
		since, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)

		h.Track(playerID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(h.History(playerID, since)) //nolint:errcheck
	}
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
