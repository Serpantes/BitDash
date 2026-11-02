package delivery

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"bitdash/backend/internal/repository"
)

const maxDashboardBody = 1 << 20

type dashboardSaveRequest struct {
	BaseRev int64           `json:"baseRev"`
	Blocks  json.RawMessage `json:"blocks"`
	Force   bool            `json:"force"`
}

// NewDashboardHandler serves the visitor's own pinned dashboard blocks (wrap it
// in WithVisitor): GET returns {rev, updatedAt, blocks}; PUT {baseRev, blocks}
// saves them and answers 409 with the current state when baseRev is stale.
func NewDashboardHandler(stores *repository.DashboardStores) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		store, err := stores.For(VisitorID(r))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodGet:
			state, err := store.Load()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeDashboard(w, http.StatusOK, state)
		case http.MethodPut, http.MethodPost:
			body, err := io.ReadAll(io.LimitReader(r.Body, maxDashboardBody+1))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if len(body) > maxDashboardBody {
				http.Error(w, "dashboard too large", http.StatusRequestEntityTooLarge)
				return
			}
			var req dashboardSaveRequest
			if err := json.Unmarshal(body, &req); err != nil {
				http.Error(w, "invalid JSON", http.StatusBadRequest)
				return
			}
			state, err := store.Save(req.BaseRev, req.Blocks, req.Force)
			if errors.Is(err, repository.ErrDashboardConflict) {
				writeDashboard(w, http.StatusConflict, state)
				return
			}
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeDashboard(w, http.StatusOK, state)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func writeDashboard(w http.ResponseWriter, status int, state repository.DashboardState) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(state) //nolint:errcheck
}
