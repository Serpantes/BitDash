package delivery

import (
	"io"
	"net/http"
	"strings"

	"bitdash/backend/internal/repository"
)

type bitjitaProxyHandler struct {
	cache *repository.BitjitaCache
}

// NewBitjitaProxyHandler returns a handler that proxies /api/bitjita/* → BitJita API
// with disk cache for static data and memory cache for live endpoints.
func NewBitjitaProxyHandler(c *repository.BitjitaCache) func(http.ResponseWriter, *http.Request) {
	h := &bitjitaProxyHandler{cache: c}
	return h.handle
}

func (h *bitjitaProxyHandler) handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/bitjita")
	if path == "" {
		path = "/"
	}
	query := r.URL.RawQuery

	if r.Method == http.MethodGet {
		body, status, cacheStatus, err := h.cache.GetAPI(path, query)
		if err != nil {
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", cacheStatus)
		if repository.BitjitaCachePolicy(path, query) == "permanent" {
			w.Header().Set("Cache-Control", "public, max-age=86400")
		}
		w.WriteHeader(status)
		w.Write(body) //nolint:errcheck
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	respBody, status, err := h.cache.PostAPI(path, query, body)
	if err != nil {
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.WriteHeader(status)
	w.Write(respBody) //nolint:errcheck
}
