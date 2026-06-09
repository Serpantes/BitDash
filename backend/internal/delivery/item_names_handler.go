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

func NewItemNamesHandler(baseURL string, store *repository.ItemNameStore) http.HandlerFunc {
	h := &itemNamesHandler{
		store:      store,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
	return h.serveHTTP
}

type itemNameRequest struct {
	ID   int64 `json:"id"`
	Type int   `json:"type"`
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

	result := make(map[string]repository.ItemMeta, len(items))
	var missing []itemNameRequest
	for _, item := range items {
		key := repository.ItemKey(item.Type, item.ID)
		if meta, ok := h.store.Get(key); ok && meta.Name != "" && meta.IconAssetName != "" && meta.RarityStr != "" {
			result[key] = meta
		} else {
			missing = append(missing, item)
		}
	}

	newEntries := make(map[string]repository.ItemMeta, len(missing))
	for _, item := range missing {
		key := repository.ItemKey(item.Type, item.ID)
		meta := h.fetchMeta(item)
		result[key] = meta
		newEntries[key] = meta
	}
	if len(newEntries) > 0 {
		_ = h.store.SetMany(newEntries)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result) //nolint:errcheck
}

func (h *itemNamesHandler) fetchMeta(item itemNameRequest) repository.ItemMeta {
	url := fmt.Sprintf("%s/api/items/%d", h.baseURL, item.ID)
	topKey := "item"
	if item.Type == 1 {
		url = fmt.Sprintf("%s/api/cargo/%d", h.baseURL, item.ID)
		topKey = "cargo"
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return repository.ItemMeta{Name: fmt.Sprintf("#%d", item.ID)}
	}
	req.Header.Set("x-app-identifier", "BitDash")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return repository.ItemMeta{Name: fmt.Sprintf("#%d", item.ID)}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return repository.ItemMeta{Name: fmt.Sprintf("#%d", item.ID)}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return repository.ItemMeta{Name: fmt.Sprintf("#%d", item.ID)}
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return repository.ItemMeta{Name: fmt.Sprintf("#%d", item.ID)}
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(envelope[topKey], &obj); err != nil {
		return repository.ItemMeta{Name: fmt.Sprintf("#%d", item.ID)}
	}

	meta := repository.ItemMeta{Name: fmt.Sprintf("#%d", item.ID)}
	_ = json.Unmarshal(obj["name"], &meta.Name)
	_ = json.Unmarshal(obj["iconAssetName"], &meta.IconAssetName)
	_ = json.Unmarshal(obj["tier"], &meta.Tier)
	_ = json.Unmarshal(obj["rarityStr"], &meta.RarityStr)
	return meta
}
