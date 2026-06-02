package delivery

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"bitdash/backend/internal/repository"
)

type itemNamesHandler struct {
	store      *repository.ItemNameStore
	baseURL    string
	httpClient *http.Client
}

// NewItemNamesHandler returns an HTTP handler for POST /api/item-names.
// It resolves item/cargo display names, caching results in store.
func NewItemNamesHandler(baseURL string, store *repository.ItemNameStore) http.HandlerFunc {
	h := &itemNamesHandler{
		store:   store,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
	return h.serveHTTP
}

type itemNameRequest struct {
	ID   int64 `json:"id"`
	Type int   `json:"type"` // 0 = item, 1 = cargo
}

func (h *itemNamesHandler) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var items []itemNameRequest
	if err := json.NewDecoder(r.Body).Decode(&items); err != nil || len(items) == 0 {
		http.Error(w, "invalid body: expected non-empty JSON array", http.StatusBadRequest)
		return
	}

	result := make(map[string]string, len(items))
	var missing []itemNameRequest

	for _, item := range items {
		key := repository.ItemKey(item.Type, item.ID)
		if name, ok := h.store.Get(key); ok {
			result[key] = name
		} else {
			missing = append(missing, item)
		}
	}

	newEntries := make(map[string]string, len(missing))
	for _, item := range missing {
		key := repository.ItemKey(item.Type, item.ID)
		name := h.fetchName(item)
		result[key] = name
		newEntries[key] = name
	}

	if len(newEntries) > 0 {
		_ = h.store.SetMany(newEntries) // best-effort; don't fail the request on disk error
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result) //nolint:errcheck
}

func (h *itemNamesHandler) fetchName(item itemNameRequest) string {
	var url string
	if item.Type == 1 {
		url = fmt.Sprintf("%s/api/cargo/%d", h.baseURL, item.ID)
	} else {
		url = fmt.Sprintf("%s/api/items/%d", h.baseURL, item.ID)
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Sprintf("#%d", item.ID)
	}
	req.Header.Set("x-app-identifier", "BitDash")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Sprintf("#%d", item.ID)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("#%d", item.ID)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("#%d", item.ID)
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Sprintf("#%d", item.ID)
	}

	topKey := "item"
	if item.Type == 1 {
		topKey = "cargo"
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(envelope[topKey], &obj); err != nil {
		return fmt.Sprintf("#%d", item.ID)
	}

	var name string
	if err := json.Unmarshal(obj["name"], &name); err != nil {
		return fmt.Sprintf("#%d", item.ID)
	}
	return name
}
