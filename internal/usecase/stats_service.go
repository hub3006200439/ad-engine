package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// StatsOverview — DTO, который отдаём наружу.
type StatsOverview struct {
	CampaignID  uuid.UUID `json:"campaign_id"`
	Impressions int64     `json:"impressions"`
	Clicks      int64     `json:"clicks"`
	CTR         float64   `json:"ctr"`
	SpendMicros int64     `json:"spend_micros"`
}

// StatsRepository — контракт для слоя хранения.
type StatsRepository interface {
	Overview(ctx context.Context, from, to time.Time, campaignID *uuid.UUID) (*StatsOverview, error)
}

type StatsService struct {
	repo StatsRepository
}

func NewStatsService(r StatsRepository) *StatsService {
	return &StatsService{repo: r}
}

func (s *StatsService) Overview(ctx context.Context, from, to time.Time, campaignID *uuid.UUID) (*StatsOverview, error) {
	return s.repo.Overview(ctx, from, to, campaignID)
}
