package delivery

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"bitdash/backend/internal/cache"
)

func NewBitjitaIconHandler(baseURL string, c *cache.Cache) http.HandlerFunc {
	client := &http.Client{Timeout: 20 * time.Second}
	return func(w http.ResponseWriter, r *http.Request) {
		iconAssetName := strings.TrimSpace(r.URL.Query().Get("iconAssetName"))
		if iconAssetName == "" {
			http.Error(w, "iconAssetName is required", http.StatusBadRequest)
			return
		}

		cacheKey := "icon:" + strings.TrimLeft(iconAssetName, "/")
		if cached, ok := c.Get(cacheKey); ok {
			w.Header().Set("Content-Type", "image/webp")
			w.Write(cached) //nolint:errcheck
			return
		}

		data, contentType, err := fetchIcon(client, baseURL, iconAssetName)
		if err != nil {
			http.Error(w, "icon not found", http.StatusNotFound)
			return
		}
		c.Set(cacheKey, data)
		w.Header().Set("Content-Type", contentType)
		w.Write(data) //nolint:errcheck
	}
}

func fetchIcon(client *http.Client, baseURL string, iconAssetName string) ([]byte, string, error) {
	for _, assetPath := range iconAssetPathCandidates(iconAssetName) {
		assetURL, err := iconAssetURL(baseURL, assetPath)
		if err != nil {
			continue
		}
		req, err := http.NewRequest(http.MethodGet, assetURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("x-app-identifier", "BitDash")
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		data, contentType, ok := readIconResponse(resp)
		if ok {
			return data, contentType, nil
		}
	}
	return nil, "", fmt.Errorf("icon not found")
}

func iconAssetPathCandidates(iconAssetName string) []string {
	assetPath := "/" + strings.TrimLeft(strings.TrimSpace(iconAssetName), "/")
	if path.Ext(assetPath) == "" {
		assetPath += ".webp"
	}

	candidates := make([]string, 0, 2)
	if !strings.HasPrefix(strings.TrimLeft(assetPath, "/"), "GeneratedIcons/") {
		candidates = append(candidates, "/GeneratedIcons"+assetPath)
	}
	candidates = append(candidates, assetPath)
	return candidates
}

func iconAssetURL(baseURL string, assetPath string) (string, error) {
	base, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return "", err
	}
	asset := &url.URL{Path: assetPath}
	return base.ResolveReference(asset).String(), nil
}

func readIconResponse(resp *http.Response) ([]byte, string, bool) {
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
