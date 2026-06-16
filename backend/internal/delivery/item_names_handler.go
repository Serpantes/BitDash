package delivery

import (
	"encoding/json"
	"fmt"
	"net/http"

	"bitdash/backend/internal/repository"
)

type itemNamesHandler struct {
	store   *repository.ItemNameStore
	bitjita *repository.BitjitaCache
}

func NewItemNamesHandler(bitjita *repository.BitjitaCache, store *repository.ItemNameStore) http.HandlerFunc {
	h := &itemNamesHandler{
		store:   store,
		bitjita: bitjita,
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
		if meta, ok := h.store.Get(key); ok && meta.Name != "" && meta.IconAssetName != "" {
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
	path := fmt.Sprintf("/items/%d", item.ID)
	topKey := "item"
	if item.Type == 1 {
		path = fmt.Sprintf("/cargo/%d", item.ID)
		topKey = "cargo"
	}

	body, status, _, err := h.bitjita.GetAPI(path, "")
	if err != nil || status != http.StatusOK {
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
