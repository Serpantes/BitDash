package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"bitdash/backend/internal/usecase"
)

type bitjitaPlayerSource struct {
	baseURL string
	client  *http.Client
}

// NewBitjitaPlayerSource creates adapter for Bitjita external API.
func NewBitjitaPlayerSource(baseURL string) usecase.PlayerSource {
	cleaned := strings.TrimRight(baseURL, "/")
	return &bitjitaPlayerSource{
		baseURL: cleaned,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *bitjitaPlayerSource) GetPlayerByID(ctx context.Context, request usecase.PlayerByIDRequest) (*usecase.PlayerByIDResponse, error) {
	if strings.TrimSpace(request.ID) == "" {
		return nil, fmt.Errorf("player id is required")
	}

	url := fmt.Sprintf("%s/api/players/%s", s.baseURL, request.ID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("bitjita returned status %d", resp.StatusCode)
	}

	var result usecase.PlayerByIDResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}
