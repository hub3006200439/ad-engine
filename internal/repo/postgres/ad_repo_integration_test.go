//go:build integration

package postgres

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"ad-engine/internal/usecase"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// setupTestDB подключается к тестовой БД через ENV TEST_PG_DSN.
// Пример:
//
//	export TEST_PG_DSN="postgres://adengine_user:L$56dgBN109!3@localhost:5432/adengine?sslmode=disable"
func setupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("TEST_PG_DSN")
	if dsn == "" {
		t.Skip("TEST_PG_DSN not set, skipping integration test")
	}

	// Таймаут на создание пула/подключение
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)

	// Явно проверяем подключение, чтобы не было "тихого" зависания на первом запросе.
	require.NoError(t, pool.Ping(ctx))

	return pool
}

// TestAdRepository_ConcurrentBudgetSpending_NoOverspend
// проверяет, что при множественных конкурентных RequestAd
// бюджет кампании НЕ уходит за пределы budget_total/budget_daily.
func TestAdRepository_ConcurrentBudgetSpending_NoOverspend(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// Таймаут на весь тест (включая все запросы к БД)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Чистим таблицы, чтобы тест был детерминированным.
	_, err := pool.Exec(ctx, `
        TRUNCATE TABLE spend_events RESTART IDENTITY CASCADE;
        TRUNCATE TABLE impressions RESTART IDENTITY CASCADE;
        TRUNCATE TABLE clicks RESTART IDENTITY CASCADE;
        TRUNCATE TABLE views RESTART IDENTITY CASCADE;
        TRUNCATE TABLE user_campaign_daily RESTART IDENTITY CASCADE;
        TRUNCATE TABLE creatives RESTART IDENTITY CASCADE;
        TRUNCATE TABLE campaigns RESTART IDENTITY CASCADE;
    `)
	require.NoError(t, err)

	campID := uuid.New()
	creativeID := uuid.New()

	const (
		bidMicros    int64 = 1_000_000
		budgetTotal  int64 = 10
		budgetDaily  int64 = 10
		frequencyCap       = 0
	)

	// ВАЖНО: указываем name, т.к. в схеме campaigns.name NOT NULL.
	_, err = pool.Exec(ctx, `
        INSERT INTO campaigns (
            id,
            name,
            pricing_model,
            bid_micros,
            budget_total,
            budget_daily,
            spent_total,
            spent_today,
            spent_today_date,
            active,
            start_at,
            end_at,
            targeting_json,
            frequency_cap
        ) VALUES (
            $1,
            $2,
            'CPM',
            $3,
            $4,
            $5,
            0,
            0,
            CURRENT_DATE,
            TRUE,
            NOW() - INTERVAL '1 day',
            NOW() + INTERVAL '1 day',
            '{"languages":["en"],"countries":["US"],"categories":["tech"],"placements":["pre-roll"]}'::jsonb,
            $6
        )
    `, campID, "test-campaign", bidMicros, budgetTotal, budgetDaily, frequencyCap)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
        INSERT INTO creatives (
            id,
            campaign_id,
            duration_sec,
            video_url,
            landing_url,
            lang,
            placement,
            category
        ) VALUES (
            $1,
            $2,
            30,
            'https://cdn.example.com/test.mp4',
            'https://example.com/landing',
            'en',
            'pre-roll',
            'tech'
        )
    `, creativeID, campID)
	require.NoError(t, err)

	freqRepo := NewFreqRepository(pool)
	freqSvc := usecase.NewFrequencyService(freqRepo)
	adRepo := NewAdRepository(pool, freqSvc)

	const workers = 50

	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()

			// Контекст для каждого запроса — с тем же дедлайном,
			// чтобы зависание одного запроса не повисло навсегда.
			reqCtx, cancelReq := context.WithTimeout(ctx, 5*time.Second)
			defer cancelReq()

			_, _ = adRepo.RequestAd(reqCtx, usecase.AdRequest{
				UserID:    uuid.NewString(),
				SessionID: uuid.NewString(),
				Lang:      "en",
				Country:   "US",
				Category:  "tech",
				Placement: "pre-roll",
			})
		}(i)
	}

	wg.Wait()

	// Даём транзакциям завершиться (на всякий случай)
	time.Sleep(200 * time.Millisecond)

	var spentTotal int64
	var spentToday int64

	err = pool.QueryRow(ctx, `
        SELECT spent_total, spent_today
        FROM campaigns
        WHERE id = $1
    `, campID).Scan(&spentTotal, &spentToday)
	require.NoError(t, err)

	require.LessOrEqual(t, spentTotal, budgetTotal,
		"spent_total must NOT exceed campaign.budget_total")

	require.LessOrEqual(t, spentToday, budgetDaily,
		"spent_today must NOT exceed campaign.budget_daily")
}
