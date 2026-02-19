package usecase

import (
	"context"
	"encoding/json"
)

// PlayerByIDRequest is an application request model for fetching a player by id.
type PlayerByIDRequest struct {
	ID string `json:"id"`
}

// PlayerByIDResponse is an application response model for external player payload.
// Unknown nested structures are preserved as raw JSON to keep source adapters flexible.
type PlayerByIDResponse struct {
	Player       json.RawMessage   `json:"player"`
	Claims       []json.RawMessage `json:"claims"`
	Empires      []json.RawMessage `json:"empires"`
	MarketOrders []json.RawMessage `json:"marketOrders"`
	Skills       []json.RawMessage `json:"skills"`
}

// PlayerSource defines a replaceable data source boundary for player retrieval.
type PlayerSource interface {
	GetPlayerByID(ctx context.Context, request PlayerByIDRequest) (*PlayerByIDResponse, error)
}

// PlayerResponseStore defines persistence boundary for storing fetched player payloads.
type PlayerResponseStore interface {
	SavePlayerByIDResponse(ctx context.Context, request PlayerByIDRequest, response *PlayerByIDResponse) error
}

// PlayerService defines use-case boundary for delivery layer.
type PlayerService interface {
	GetPlayerByID(ctx context.Context, request PlayerByIDRequest) (*PlayerByIDResponse, error)
}

type playerService struct {
	source PlayerSource
	store  PlayerResponseStore
}

// NewPlayerService creates use-case service backed by abstract source adapter.
func NewPlayerService(source PlayerSource, store PlayerResponseStore) PlayerService {
	return &playerService{source: source, store: store}
}

func (s *playerService) GetPlayerByID(ctx context.Context, request PlayerByIDRequest) (*PlayerByIDResponse, error) {
	response, err := s.source.GetPlayerByID(ctx, request)
	if err != nil {
		return nil, err
	}
	if s.store != nil {
		if err := s.store.SavePlayerByIDResponse(ctx, request, response); err != nil {
			return nil, err
		}
	}
	return response, nil
}
