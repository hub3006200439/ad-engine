package postgres

import (
	"context"
	"fmt"
	"time"

	"ad-engine/internal/usecase"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type StatsRepository struct {
	db *pgxpool.Pool
}

func NewStatsRepository(pool *pgxpool.Pool) *StatsRepository {
	return &StatsRepository{db: pool}
}

func (r *StatsRepository) Overview(ctx context.Context, from, to time.Time, campaignID *uuid.UUID) (*usecase.StatsOverview, error) {
	var (
		impressions int64
		clicks      int64
		spend       int64
		cid         uuid.UUID
	)

	if campaignID != nil {
		cid = *campaignID

		// impressions + clicks
		if err := r.db.QueryRow(ctx, `
            SELECT
                COUNT(*) AS impressions,
                COUNT(c.id) AS clicks
            FROM impressions i
            LEFT JOIN clicks c ON c.impression_id = i.id
            WHERE i.created_at BETWEEN $1 AND $2
              AND i.campaign_id = $3
        `, from, to, *campaignID).Scan(&impressions, &clicks); err != nil {
			return nil, fmt.Errorf("stats impressions/clicks (campaign): %w", err)
		}

		// spend
		if err := r.db.QueryRow(ctx, `
            SELECT COALESCE(SUM(amount_micros), 0)
            FROM spend_events
            WHERE occurred_at BETWEEN $1 AND $2
              AND campaign_id = $3
        `, from, to, *campaignID).Scan(&spend); err != nil {
			return nil, fmt.Errorf("stats spend (campaign): %w", err)
		}
	} else {
		cid = uuid.Nil

		if err := r.db.QueryRow(ctx, `
            SELECT
                COUNT(*) AS impressions,
                COUNT(c.id) AS clicks
            FROM impressions i
            LEFT JOIN clicks c ON c.impression_id = i.id
            WHERE i.created_at BETWEEN $1 AND $2
        `, from, to).Scan(&impressions, &clicks); err != nil {
			return nil, fmt.Errorf("stats impressions/clicks (global): %w", err)
		}

		if err := r.db.QueryRow(ctx, `
            SELECT COALESCE(SUM(amount_micros), 0)
            FROM spend_events
            WHERE occurred_at BETWEEN $1 AND $2
        `, from, to).Scan(&spend); err != nil {
			return nil, fmt.Errorf("stats spend (global): %w", err)
		}
	}

	var ctr float64
	if impressions > 0 {
		ctr = float64(clicks) * 100.0 / float64(impressions)
	}

	return &usecase.StatsOverview{
		CampaignID:  cid,
		Impressions: impressions,
		Clicks:      clicks,
		CTR:         ctr,
		SpendMicros: spend,
	}, nil
}
