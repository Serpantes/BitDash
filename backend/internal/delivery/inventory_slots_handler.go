package delivery

import (
	"encoding/json"
	"net/http"

	"bitdash/backend/internal/repository"
)

// NewInventorySlotsHandler saves a pocket layout arranged by hand on the dashboard:
// POST {"inventoryId": "...", "items": [{slot, itemId, quantity}...], "cargo": [...]}.
func NewInventorySlotsHandler(slots *repository.InventorySlots) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			InventoryID string                     `json:"inventoryId"`
			Items       []repository.SlotPlacement `json:"items"`
			Cargo       []repository.SlotPlacement `json:"cargo"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if err := slots.SetLayout(req.InventoryID, req.Items, req.Cargo); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
