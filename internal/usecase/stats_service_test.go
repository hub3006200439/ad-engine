package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Мок репозитория статистики.
type mockStatsRepository struct {
	mock.Mock
}

func (m *mockStatsRepository) Overview(
	ctx context.Context,
	from, to time.Time,
	campaignID *uuid.UUID,
) (*StatsOverview, error) {
	args := m.Called(ctx, from, to, campaignID)
	// *StatsOverview (может быть nil)
	if v, ok := args.Get(0).(*StatsOverview); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}

//     Overview (success)

func TestStatsService_Overview_Success(t *testing.T) {
	//  - сервис вызывает repo.Overview с теми же параметрами
	//  - получает *StatsOverview
	//  - возвращает его без изменений

	ctx := context.Background()
	mockRepo := &mockStatsRepository{}
	svc := NewStatsService(mockRepo)

	from := time.Now().Add(-24 * time.Hour).UTC()
	to := time.Now().UTC()
	campID := uuid.New()

	expected := &StatsOverview{
		CampaignID:  campID,
		Impressions: 1000,
		Clicks:      123,
		CTR:         0.123,
		SpendMicros: 42_000_000,
	}

	// Настраиваем ожидание на мок:
	mockRepo.
		On("Overview", mock.Anything, from, to, &campID).
		Return(expected, nil)

	got, err := svc.Overview(ctx, from, to, &campID)

	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, expected.CampaignID, got.CampaignID)
	assert.Equal(t, expected.Impressions, got.Impressions)
	assert.Equal(t, expected.Clicks, got.Clicks)
	assert.Equal(t, expected.SpendMicros, got.SpendMicros)
	assert.InDelta(t, expected.CTR, got.CTR, 1e-9)

	mockRepo.AssertExpectations(t)
}

//     Overview (error)

func TestStatsService_Overview_Error(t *testing.T) {
	//  - сервис возвращает ошибку
	//  - *StatsOverview == nil

	ctx := context.Background()
	mockRepo := &mockStatsRepository{}
	svc := NewStatsService(mockRepo)

	from := time.Now().Add(-24 * time.Hour).UTC()
	to := time.Now().UTC()

	// Случай без campaign_id — передаём nil
	var campID *uuid.UUID = nil
	mockErr := errors.New("db error")

	mockRepo.
		On("Overview", mock.Anything, from, to, campID).
		Return((*StatsOverview)(nil), mockErr)

	got, err := svc.Overview(ctx, from, to, campID)

	require.Error(t, err)
	assert.ErrorIs(t, err, mockErr)
	assert.Nil(t, got)

	mockRepo.AssertExpectations(t)
}
