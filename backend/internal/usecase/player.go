package usecase

import (
	"context"
	"encoding/json"
)

type PlayerByIDRequest struct {
	ID string `json:"id"`
}

type PlayerByIDResponse struct {
	Player       json.RawMessage   `json:"player"`
	Claims       []json.RawMessage `json:"claims"`
	Empires      []json.RawMessage `json:"empires"`
	MarketOrders []json.RawMessage `json:"marketOrders"`
	Skills       []json.RawMessage `json:"skills"`
}

type PlayerSource interface {
	GetPlayerByID(ctx context.Context, request PlayerByIDRequest) (*PlayerByIDResponse, error)
}

type PlayerService interface {
	GetPlayerByID(ctx context.Context, request PlayerByIDRequest) (*PlayerByIDResponse, error)
}

type playerService struct {
	source PlayerSource
}

func NewPlayerService(source PlayerSource) PlayerService {
	return &playerService{source: source}
}

func (s *playerService) GetPlayerByID(ctx context.Context, request PlayerByIDRequest) (*PlayerByIDResponse, error) {
	return s.source.GetPlayerByID(ctx, request)
}
