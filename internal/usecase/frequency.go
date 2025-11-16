package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// FrequencyRepository — Postgres-репозиторий, хранящий freq cap.
type FrequencyRepository interface {
	// frequency_cap кампании
	GetCampaignLimit(ctx context.Context, campaignID uuid.UUID) (int, error)

	// количество показов за день
	GetDailyImpressions(ctx context.Context,
		userID, campaignID uuid.UUID, day time.Time,
	) (int, error)

	// инкремент счётчика показов
	IncDailyImpressions(ctx context.Context,
		userID, campaignID uuid.UUID, day time.Time,
	) error
}

type FrequencyService struct {
	repo FrequencyRepository
}

func NewFrequencyService(repo FrequencyRepository) *FrequencyService {
	return &FrequencyService{repo: repo}
}

// Allowed — можно ли показать кампанию user'у сегодня?
func (f *FrequencyService) Allowed(
	ctx context.Context,
	userID, campaignID uuid.UUID,
) (bool, error) {
	limit, err := f.repo.GetCampaignLimit(ctx, campaignID)
	if err != nil {
		return false, err
	}

	// 0 или отрицательное значение — без ограничения.
	if limit <= 0 {
		return true, nil
	}

	day := time.Now().UTC().Truncate(24 * time.Hour)

	current, err := f.repo.GetDailyImpressions(ctx, userID, campaignID, day)
	if err != nil {
		return false, err
	}

	return current < limit, nil
}

// Register — фиксирует показ (после успешного impression).
func (f *FrequencyService) Register(
	ctx context.Context,
	userID, campaignID uuid.UUID,
) error {
	day := time.Now().UTC().Truncate(24 * time.Hour)
	return f.repo.IncDailyImpressions(ctx, userID, campaignID, day)
}
