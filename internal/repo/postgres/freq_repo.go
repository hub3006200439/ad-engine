package postgres

import (
	"context"
	"time"

	"ad-engine/internal/usecase"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FreqRepository struct {
	db *pgxpool.Pool
}

var _ usecase.FrequencyRepository = (*FreqRepository)(nil)

func NewFreqRepository(db *pgxpool.Pool) *FreqRepository {
	return &FreqRepository{db: db}
}

// GetCampaignLimit — frequency_cap из campaigns.
func (r *FreqRepository) GetCampaignLimit(
	ctx context.Context,
	campaignID uuid.UUID,
) (int, error) {
	const q = `
        SELECT frequency_cap
        FROM campaigns
        WHERE id = $1
    `

	var limit int
	err := r.db.QueryRow(ctx, q, campaignID).Scan(&limit)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	return limit, nil
}

// GetDailyImpressions — количество показов за текущий день.
func (r *FreqRepository) GetDailyImpressions(
	ctx context.Context,
	userID, campaignID uuid.UUID,
	day time.Time,
) (int, error) {
	const q = `
        SELECT impressions
        FROM user_campaign_daily
        WHERE user_id = $1
          AND campaign_id = $2
          AND day = $3
    `
	var n int
	err := r.db.QueryRow(ctx, q, userID, campaignID, day).Scan(&n)
	if err == pgx.ErrNoRows {
		return 0, nil
	}
	return n, err
}

// IncDailyImpressions — атомарный INSERT/UPDATE в одной транзакции.
func (r *FreqRepository) IncDailyImpressions(
	ctx context.Context,
	userID, campaignID uuid.UUID,
	day time.Time,
) error {
	const q = `
        INSERT INTO user_campaign_daily (user_id, campaign_id, day, impressions)
        VALUES ($1, $2, $3, 1)
        ON CONFLICT (user_id, campaign_id, day)
        DO UPDATE SET impressions = user_campaign_daily.impressions + 1
    `
	_, err := r.db.Exec(ctx, q, userID, campaignID, day)
	return err
}
