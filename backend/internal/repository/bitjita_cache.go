package repository

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"bitdash/backend/internal/cache"
)

const bitjitaAppID = "BitDash"

type BitjitaCache struct {
	baseURL    string
	client     *http.Client
	apiFiles   *FileCache
	iconFiles  *FileCache
	liveMemory *cache.Cache

	// Background refreshes of "stale" paths currently in flight, keyed by memory key.
	staleRefreshing sync.Map
	// How long a no-cache request on a "stale" path waits for BitJita before taking the stored answer.
	staleFreshWait time.Duration
}

type staleFlight struct {
	done chan struct{}
	body []byte
	ok   bool
}

func NewBitjitaCache(baseURL, cacheRoot string, liveMemory *cache.Cache) (*BitjitaCache, error) {
	apiFiles, err := NewFileCache(filepath.Join(cacheRoot, "api"))
	if err != nil {
		return nil, err
	}
	iconFiles, err := NewFileCache(filepath.Join(cacheRoot, "icons"))
	if err != nil {
		return nil, err
	}
	return &BitjitaCache{
		baseURL:        strings.TrimRight(baseURL, "/"),
		client:         &http.Client{Timeout: 120 * time.Second},
		apiFiles:       apiFiles,
		iconFiles:      iconFiles,
		liveMemory:     liveMemory,
		staleFreshWait: 3 * time.Second,
	}, nil
}

func BitjitaCachePolicy(path, query string) string {
	return bitjitaCachePolicy(path, query)
}

func bitjitaCachePolicy(path, query string) string {
	if isStaticPath(path) {
		return "permanent"
	}
	if path == "/items" && query != "" {
		return "permanent"
	}
	if path == "/cargo" && query != "" {
		return "permanent"
	}
	if isNumericDetailPath(path, "/items/") || isNumericDetailPath(path, "/cargo/") {
		return "permanent"
	}
	if isSlowRankingPath(path) {
		return "stale"
	}
	return "live"
}

// BitJita computes skill rankings and leaderboards on demand and can take close to a minute,
// so the last good answer is served at once while a fresh one is fetched in the background.
func isSlowRankingPath(path string) bool {
	if path == "/leaderboard/skills" {
		return true
	}
	return strings.HasPrefix(path, "/players/") && strings.HasSuffix(path, "/skill-rankings")
}

func staleDiskKey(path, query string) string {
	return "stale_" + apiDiskKey(path, query)
}

func isStaticPath(path string) bool {
	return strings.HasPrefix(path, "/static/")
}

func bitjitaUpstreamPath(path string) string {
	if isStaticPath(path) {
		return path
	}
	return "/api" + path
}

func isNumericDetailPath(path, prefix string) bool {
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	rest := strings.TrimPrefix(path, prefix)
	if rest == "" || strings.Contains(rest, "/") {
		return false
	}
	for _, c := range rest {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func apiDiskKey(path, query string) string {
	p := strings.Trim(strings.TrimPrefix(path, "/"), "/")
	p = strings.ReplaceAll(p, "/", "_")
	if query == "" {
		return p + ".json"
	}
	q := strings.ReplaceAll(query, "/", "_")
	q = strings.ReplaceAll(q, "&", "_")
	return p + "_" + q + ".json"
}

func apiMemoryKey(path, query string) string {
	if query == "" {
		return path
	}
	return path + "?" + query
}

func (b *BitjitaCache) GetAPI(path, query string) (body []byte, status int, cacheStatus string, err error) {
	return b.getAPI(path, query, false)
}

func (b *BitjitaCache) GetAPIFresh(path, query string) (body []byte, status int, cacheStatus string, err error) {
	return b.getAPI(path, query, true)
}

func (b *BitjitaCache) getAPI(path, query string, fresh bool) (body []byte, status int, cacheStatus string, err error) {
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	policy := bitjitaCachePolicy(path, query)
	diskKey := apiDiskKey(path, query)
	memKey := apiMemoryKey(path, query)

	if policy == "permanent" {
		if data, ok := b.apiFiles.Get(diskKey); ok {
			return data, http.StatusOK, "DISK-HIT", nil
		}
	} else if !fresh {
		if data, ok := b.liveMemory.Get(memKey); ok {
			return data, http.StatusOK, "HIT", nil
		}
	}

	if policy == "stale" {
		staleKey := staleDiskKey(path, query)
		stale, haveStale := b.apiFiles.Get(staleKey)
		if haveStale {
			flight := b.refreshStale(path, query, memKey, staleKey)
			if !fresh {
				return stale, http.StatusOK, "STALE", nil
			}
			select {
			case <-flight.done:
				if flight.ok {
					return flight.body, http.StatusOK, "MISS", nil
				}
			case <-time.After(b.staleFreshWait):
			}
			return stale, http.StatusOK, "STALE", nil
		}
		body, status, err = b.fetchUpstream(path, query)
		if err == nil && status >= 200 && status < 300 {
			b.liveMemory.Set(memKey, body)
			_ = b.apiFiles.Set(staleKey, body)
			return body, status, "MISS", nil
		}
		if haveStale {
			return stale, http.StatusOK, "STALE", nil
		}
		return body, status, "MISS", err
	}

	body, status, err = b.fetchUpstream(path, query)
	if err != nil {
		return nil, status, "", err
	}

	if status >= 200 && status < 300 {
		if policy == "permanent" {
			_ = b.apiFiles.Set(diskKey, body)
		} else {
			b.liveMemory.Set(memKey, body)
		}
	}
	return body, status, "MISS", nil
}

func (b *BitjitaCache) refreshStale(path, query, memKey, staleKey string) *staleFlight {
	flight := &staleFlight{done: make(chan struct{})}
	if running, loaded := b.staleRefreshing.LoadOrStore(memKey, flight); loaded {
		return running.(*staleFlight)
	}
	go func() {
		defer func() {
			b.staleRefreshing.Delete(memKey)
			close(flight.done)
		}()
		body, status, err := b.fetchUpstream(path, query)
		if err != nil || status < 200 || status >= 300 {
			return
		}
		b.liveMemory.Set(memKey, body)
		_ = b.apiFiles.Set(staleKey, body)
		flight.body, flight.ok = body, true
	}()
	return flight
}

func (b *BitjitaCache) fetchUpstream(path, query string) ([]byte, int, error) {
	targetURL := b.baseURL + bitjitaUpstreamPath(path)
	if query != "" {
		targetURL += "?" + query
	}

	req, err := http.NewRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("x-app-identifier", bitjitaAppID)

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

func (b *BitjitaCache) PostAPI(path string, query string, jsonBody []byte) ([]byte, int, error) {
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	memKey := apiMemoryKey(path, query)
	if path == "/market/prices/bulk" && query == "" {
		memKey = path + "\n" + string(jsonBody)
		if data, ok := b.liveMemory.Get(memKey); ok {
			return data, http.StatusOK, nil
		}
	}

	targetURL := b.baseURL + "/api" + path
	if query != "" {
		targetURL += "?" + query
	}

	req, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("x-app-identifier", bitjitaAppID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	if path == "/market/prices/bulk" && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		b.liveMemory.Set(memKey, body)
	}
	return body, resp.StatusCode, nil
}

func (b *BitjitaCache) GetIcon(iconAssetName string) ([]byte, string, error) {
	key := "icon:" + strings.TrimLeft(strings.TrimSpace(iconAssetName), "/")
	if data, ok := b.iconFiles.Get(key); ok {
		return data, "image/webp", nil
	}

	data, contentType, err := fetchBitjitaIcon(b.client, b.baseURL, iconAssetName)
	if err != nil {
		return nil, "", err
	}
	_ = b.iconFiles.Set(key, data)
	return data, contentType, nil
}

func fetchBitjitaIcon(client *http.Client, baseURL, iconAssetName string) ([]byte, string, error) {
	for _, assetPath := range iconAssetPathCandidates(iconAssetName) {
		assetURL, err := iconAssetURL(baseURL, assetPath)
		if err != nil {
			continue
		}
		req, err := http.NewRequest(http.MethodGet, assetURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("x-app-identifier", bitjitaAppID)
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		data, contentType, ok := readIconHTTPResponse(resp)
		if ok {
			return data, contentType, nil
		}
	}
	return nil, "", fmt.Errorf("icon not found")
}

func iconAssetPathCandidates(iconAssetName string) []string {
	assetPath := "/" + strings.TrimLeft(strings.TrimSpace(iconAssetName), "/")
	if !strings.HasSuffix(strings.ToLower(assetPath), ".webp") {
		assetPath += ".webp"
	}

	candidates := make([]string, 0, 2)
	if !strings.HasPrefix(strings.TrimLeft(assetPath, "/"), "GeneratedIcons/") {
		candidates = append(candidates, "/GeneratedIcons"+assetPath)
	}
	candidates = append(candidates, assetPath)
	return candidates
}

func iconAssetURL(baseURL, assetPath string) (string, error) {
	base := strings.TrimRight(baseURL, "/")
	return base + assetPath, nil
}

func readIconHTTPResponse(resp *http.Response) ([]byte, string, bool) {
	defer resp.Body.Close()
	contentType := resp.Header.Get("Content-Type")
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(contentType, "image/") {
		return nil, "", false
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, "", false
	}
	if contentType == "" {
		contentType = "image/webp"
	}
	return data, contentType, true
}
