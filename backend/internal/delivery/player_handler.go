package delivery

import (
	"encoding/json"
	"net/http"
	"strings"

	"bitdash/backend/internal/usecase"
)

func NewPlayerHandler(service usecase.PlayerService) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		id := strings.TrimPrefix(r.URL.Path, "/api/players/")
		id = strings.TrimSpace(id)
		if id == "" || strings.Contains(id, "/") {
			http.Error(w, "invalid player id", http.StatusBadRequest)
			return
		}

		request := usecase.PlayerByIDRequest{ID: id}
		response, err := service.GetPlayerByID(r.Context(), request)
		if err != nil {
			http.Error(w, "failed to fetch player", http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
			return
		}
	}
}
