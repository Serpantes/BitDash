package delivery

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"

	"bitdash/backend/internal/cache"
)

type bitjitaProxyHandler struct {
	baseURL string
	client  *http.Client
	cache   *cache.Cache
}

// NewBitjitaProxyHandler returns a handler that proxies /api/bitjita/* → baseURL/api/*
// with in-memory caching for GET and selected POST requests.
func NewBitjitaProxyHandler(baseURL string, c *cache.Cache) func(http.ResponseWriter, *http.Request) {
	h := &bitjitaProxyHandler{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 60 * time.Second},
		cache:   c,
	}
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

	// Strip /api/bitjita prefix, keep the rest (e.g. /status, /regions/status)
	path := strings.TrimPrefix(r.URL.Path, "/api/bitjita")
	if path == "" {
		path = "/"
	}

	targetURL := h.baseURL + "/api" + path
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	var reqBody io.Reader
	cacheKey := targetURL
	if r.Method == http.MethodPost {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()
		reqBody = bytes.NewReader(body)
		if path == "/market/prices/bulk" {
			cacheKey = targetURL + "\n" + string(body)
		}
	}

	if r.Method == http.MethodGet || (r.Method == http.MethodPost && path == "/market/prices/bulk") {
		if data, ok := h.cache.Get(cacheKey); ok {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache", "HIT")
			w.Write(data)
			return
		}
	}

	req, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, reqBody)
	if err != nil {
		http.Error(w, "bad upstream request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("x-app-identifier", "BitDash")
	if ct := r.Header.Get("Content-Type"); ct != "" {
		req.Header.Set("Content-Type", ct)
	} else if r.Method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := h.client.Do(req)
	if err != nil {
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "failed to read upstream response", http.StatusBadGateway)
		return
	}

	if (r.Method == http.MethodGet || (r.Method == http.MethodPost && path == "/market/prices/bulk")) && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		h.cache.Set(cacheKey, respBody)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}
