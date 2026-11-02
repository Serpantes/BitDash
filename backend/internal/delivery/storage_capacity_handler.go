package delivery

import (
	"net/http"

	"bitdash/backend/internal/repository"
)

func NewStorageCapacityHandler(capacity *repository.StorageCapacity) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := capacity.Get()
		if err != nil {
			http.Error(w, "storage capacity unavailable", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(body) //nolint:errcheck
	}
}
