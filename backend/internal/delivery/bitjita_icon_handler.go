package delivery

import (
	"net/http"

	"bitdash/backend/internal/repository"
)

func NewBitjitaIconHandler(c *repository.BitjitaCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		iconAssetName := r.URL.Query().Get("iconAssetName")
		if iconAssetName == "" {
			http.Error(w, "iconAssetName is required", http.StatusBadRequest)
			return
		}

		data, contentType, err := c.GetIcon(iconAssetName)
		if err != nil {
			http.Error(w, "icon not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
		w.Write(data) //nolint:errcheck
	}
}
