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

type PlayerResponseStore interface {
	SavePlayerByIDResponse(ctx context.Context, request PlayerByIDRequest, response *PlayerByIDResponse) error
}

type PlayerService interface {
	GetPlayerByID(ctx context.Context, request PlayerByIDRequest) (*PlayerByIDResponse, error)
}

type playerService struct {
	source PlayerSource
	store  PlayerResponseStore
}

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
