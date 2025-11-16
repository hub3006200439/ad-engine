package postgres

import (
	"context"
	"fmt"
	"time"

	"ad-engine/internal/usecase"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	pricingModelCPM = "CPM"
	pricingModelCPC = "CPC"
)

type AdRepository struct {
	db   *pgxpool.Pool
	freq *usecase.FrequencyService
}

func NewAdRepository(db *pgxpool.Pool, freq *usecase.FrequencyService) *AdRepository {
	return &AdRepository{
		db:   db,
		freq: freq,
	}
}

// RequestAd:
//  1. Идемпотентность по request_id (если задан).
//  2. Старт tx.
//  3. Выбор кандидатов по таргетингу (без freq cap).
//  4. Ранжирование в памяти.
//  5. Лочим кампанию (FOR UPDATE).
//  6. Проверяем freq cap через FrequencyService.
//  7. Списываем CPM (CPC — при клике).
//  8. Пишем impression.
//  9. Commit.
//
// 10. Регистрируем freq cap (FrequencyService.Register).
func (r *AdRepository) RequestAd(ctx context.Context, req usecase.AdRequest) (*usecase.AdResponse, error) {
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user_id: %w", err)
	}
	sessionID, err := uuid.Parse(req.SessionID)
	if err != nil {
		return nil, fmt.Errorf("invalid session_id: %w", err)
	}

	// 1. Идемпотентность по request_id
	if req.RequestID != "" {
		if resp, err := r.findByRequestID(ctx, req.RequestID); err != nil {
			return nil, err
		} else if resp != nil {
			return resp, nil
		}
	}

	// 2. Транзакция
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	// 3. Кандидаты по таргетингу (без freq cap — он через FrequencyService)
	candidates, err := r.selectCandidates(ctx, tx, req)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	// 4. Ранжирование
	best := pickBestCandidate(req, candidates)
	if best == nil {
		return nil, nil
	}

	// 5. Лочим кампанию
	camp, err := r.lockCampaignForUpdate(ctx, tx, best.CampaignID)
	if err != nil {
		return nil, err
	}
	if camp == nil {
		return nil, nil
	}

	// 6. Проверяем freq cap (в БД через FrequencyService/FreqRepository)
	allowed, err := r.freq.Allowed(ctx, userID, camp.ID)
	if err != nil {
		return nil, fmt.Errorf("freq cap allowed: %w", err)
	}
	if !allowed {
		// превышен частотный лимит → no-fill
		return nil, nil
	}

	// 7. Списываем CPM (CPC — при клике)
	var costMicros int64
	switch camp.PricingModel {
	case pricingModelCPM:
		costMicros = camp.BidMicros / 1000
		if costMicros == 0 {
			costMicros = 1
		}
	case pricingModelCPC:
		costMicros = 0
	default:
		// неизвестная модель — подстраховочно no-fill
		return nil, nil
	}

	if costMicros > 0 {
		ok, err := r.updateBudgetForEvent(ctx, tx, camp, costMicros, "CPM_IMPRESSION")
		if err != nil {
			return nil, err
		}
		if !ok {
			// бюджет/лимит исчерпан → no-fill
			return nil, nil
		}
	}

	// 8. Пишем impression
	token := uuid.NewString()
	if err := r.insertImpression(ctx, tx, best, userID, sessionID, token, req.RequestID); err != nil {
		return nil, err
	}

	// 9. Commit
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	tx = nil

	// 10. Регистрируем freq cap (увеличиваем счётчик user_campaign_daily)
	if err := r.freq.Register(ctx, userID, camp.ID); err != nil {
		// для MVP можно либо вернуть ошибку, либо просто залогировать;
		// здесь возвращаем ошибку, чтобы не скрывать проблему.
		return nil, fmt.Errorf("freq cap register: %w", err)
	}

	resp := &usecase.AdResponse{
		Token:      token,
		CreativeID: best.CreativeID,
		Duration:   best.DurationSec,
		VideoURL:   best.VideoURL,
		ClickURL:   fmt.Sprintf("/ad/click/%s", token),
		ViewURL:    fmt.Sprintf("/ad/view/%s", token),
	}
	return resp, nil
}

// TrackClick — CPC биллинг + идемпотентность по impression_id.
// Возвращает landing_url (или "" если показ не найден).
func (r *AdRepository) TrackClick(ctx context.Context, token string) (string, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	var (
		impressionID uuid.UUID
		campaignID   uuid.UUID
		creativeID   uuid.UUID
		pricing      string
		bidMicros    int64
		landingURL   string
	)

	err = tx.QueryRow(ctx, `
        SELECT
            i.id,
            i.campaign_id,
            i.creative_id,
            c.pricing_model,
            c.bid_micros,
            cr.landing_url
        FROM impressions i
        JOIN campaigns c ON c.id = i.campaign_id
        JOIN creatives cr ON cr.id = i.creative_id
        WHERE i.token = $1
        FOR UPDATE OF c
    `, token).Scan(
		&impressionID,
		&campaignID,
		&creativeID,
		&pricing,
		&bidMicros,
		&landingURL,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("select impression for click: %w", err)
	}

	clickToken := uuid.NewString()
	tag, err := tx.Exec(ctx, `
        INSERT INTO clicks (impression_id, token)
        VALUES ($1, $2)
        ON CONFLICT (impression_id) DO NOTHING
    `, impressionID, clickToken)
	if err != nil {
		return "", fmt.Errorf("insert click: %w", err)
	}

	// повторный клик — идемпотентность, без биллинга
	if tag.RowsAffected() == 0 {
		if err := tx.Commit(ctx); err != nil {
			return "", fmt.Errorf("commit tx (duplicate click): %w", err)
		}
		tx = nil
		return landingURL, nil
	}

	// новый клик — для CPC списываем бюджет
	if pricing == pricingModelCPC {
		camp, err := r.lockCampaignForUpdate(ctx, tx, campaignID)
		if err != nil {
			return "", err
		}
		if camp != nil {
			_, err := r.updateBudgetForEvent(ctx, tx, camp, bidMicros, "CPC_CLICK")
			if err != nil {
				return "", err
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit tx click: %w", err)
	}
	tx = nil

	return landingURL, nil
}

// TrackView — идемпотентная запись события просмотра.
func (r *AdRepository) TrackView(ctx context.Context, token string) error {
	_, err := r.db.Exec(ctx, `
        INSERT INTO views (impression_id)
        SELECT i.id
        FROM impressions i
        WHERE i.token = $1
        ON CONFLICT (impression_id) DO NOTHING
    `, token)
	if err != nil {
		return fmt.Errorf("insert view: %w", err)
	}
	return nil
}

// ---------------- Вспомогательные структуры / SQL ----------------

type candidateRow struct {
	CampaignID   uuid.UUID
	CreativeID   uuid.UUID
	PricingModel string
	BidMicros    int64
	DurationSec  int
	VideoURL     string
	LandingURL   string
	Lang         string
	Placement    string
	Category     string
}

const selectCandidatesSQL = `
SELECT
    c.id,
    cr.id,
    c.pricing_model,
    c.bid_micros,
    cr.duration_sec,
    cr.video_url,
    cr.landing_url,
    cr.lang,
    cr.placement,
    cr.category
FROM campaigns c
JOIN creatives cr ON cr.campaign_id = c.id
WHERE
    c.active = TRUE
    AND (c.start_at IS NULL OR c.start_at <= NOW())
    AND (c.end_at   IS NULL OR c.end_at   >= NOW())
    AND (c.budget_total = 0 OR c.spent_total < c.budget_total)
    AND (c.budget_daily = 0 OR c.spent_today < c.budget_daily)
    AND (
        c.targeting_json->'languages' IS NULL
        OR (c.targeting_json->'languages') ? $1
    )
    AND (
        c.targeting_json->'countries' IS NULL
        OR (c.targeting_json->'countries') ? $2
    )
    AND (
        c.targeting_json->'categories' IS NULL
        OR (c.targeting_json->'categories') ? $3
    )
    AND (
        c.targeting_json->'placements' IS NULL
        OR (c.targeting_json->'placements') ? $4
    )
LIMIT 100
`

func (r *AdRepository) selectCandidates(ctx context.Context, tx pgx.Tx, req usecase.AdRequest) ([]candidateRow, error) {
	rows, err := tx.Query(ctx, selectCandidatesSQL,
		req.Lang,
		req.Country,
		req.Category,
		req.Placement,
	)
	if err != nil {
		return nil, fmt.Errorf("select candidates: %w", err)
	}
	defer rows.Close()

	var out []candidateRow
	for rows.Next() {
		var c candidateRow
		if err := rows.Scan(
			&c.CampaignID,
			&c.CreativeID,
			&c.PricingModel,
			&c.BidMicros,
			&c.DurationSec,
			&c.VideoURL,
			&c.LandingURL,
			&c.Lang,
			&c.Placement,
			&c.Category,
		); err != nil {
			return nil, fmt.Errorf("scan candidate: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows err: %w", err)
	}
	return out, nil
}

// pickBestCandidate — ранжирование кандидатов по bid + совпадениям.
func pickBestCandidate(req usecase.AdRequest, candidates []candidateRow) *candidateRow {
	var best *candidateRow
	var bestScore int64

	for i := range candidates {
		c := &candidates[i]
		score := c.BidMicros

		if c.Lang == req.Lang {
			score += c.BidMicros / 2
		}
		if c.Placement == req.Placement {
			score += c.BidMicros / 3
		}
		if c.Category == req.Category {
			score += c.BidMicros / 4
		}

		if best == nil || score > bestScore {
			best = c
			bestScore = score
		}
	}

	return best
}

type campaignBudgetRow struct {
	ID             uuid.UUID
	PricingModel   string
	BidMicros      int64
	BudgetTotal    int64
	BudgetDaily    int64
	SpentTotal     int64
	SpentToday     int64
	SpentTodayDate time.Time
	FrequencyCap   int // пока не используется здесь, но есть в схеме
}

func (r *AdRepository) lockCampaignForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*campaignBudgetRow, error) {
	row := tx.QueryRow(ctx, `
        SELECT
            id,
            pricing_model,
            bid_micros,
            budget_total,
            budget_daily,
            spent_total,
            spent_today,
            spent_today_date,
            frequency_cap
        FROM campaigns
        WHERE id = $1
        FOR UPDATE
    `, id)

	var c campaignBudgetRow
	if err := row.Scan(
		&c.ID,
		&c.PricingModel,
		&c.BidMicros,
		&c.BudgetTotal,
		&c.BudgetDaily,
		&c.SpentTotal,
		&c.SpentToday,
		&c.SpentTodayDate,
		&c.FrequencyCap,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan campaign for update: %w", err)
	}
	return &c, nil
}

// updateBudgetForEvent — общий метод для CPM/CPC списаний.
// Возвращает ok=false, если превышен бюджет/лимиты.
func (r *AdRepository) updateBudgetForEvent(
	ctx context.Context,
	tx pgx.Tx,
	camp *campaignBudgetRow,
	costMicros int64,
	eventType string,
) (bool, error) {
	today := time.Now().UTC().Truncate(24 * time.Hour)

	daySpent := costMicros
	if sameDate(camp.SpentTodayDate, today) {
		daySpent += camp.SpentToday
	}
	totalSpent := camp.SpentTotal + costMicros

	if camp.BudgetTotal > 0 && totalSpent > camp.BudgetTotal {
		return false, nil
	}
	if camp.BudgetDaily > 0 && daySpent > camp.BudgetDaily {
		return false, nil
	}

	_, err := tx.Exec(ctx, `
        UPDATE campaigns
        SET spent_total = $2,
            spent_today = $3,
            spent_today_date = CURRENT_DATE
        WHERE id = $1
    `, camp.ID, totalSpent, daySpent)
	if err != nil {
		return false, fmt.Errorf("update campaign budget: %w", err)
	}

	_, err = tx.Exec(ctx, `
        INSERT INTO spend_events (campaign_id, event_type, amount_micros)
        VALUES ($1, $2, $3)
    `, camp.ID, eventType, costMicros)
	if err != nil {
		return false, fmt.Errorf("insert spend_event: %w", err)
	}

	return true, nil
}

func sameDate(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func (r *AdRepository) insertImpression(
	ctx context.Context,
	tx pgx.Tx,
	best *candidateRow,
	userID, sessionID uuid.UUID,
	token, requestID string,
) error {
	impID := uuid.New()

	_, err := tx.Exec(ctx, `
        INSERT INTO impressions (id, token, request_id, campaign_id, creative_id, user_id, session_id)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
    `,
		impID,
		token,
		nullOrString(requestID),
		best.CampaignID,
		best.CreativeID,
		userID,
		sessionID,
	)
	if err != nil {
		return fmt.Errorf("insert impression: %w", err)
	}
	return nil
}

// findByRequestID — идемпотентность по request_id (возвращает уже созданный показ).
func (r *AdRepository) findByRequestID(ctx context.Context, requestID string) (*usecase.AdResponse, error) {
	if requestID == "" {
		return nil, nil
	}

	row := r.db.QueryRow(ctx, `
        SELECT
            i.token,
            i.creative_id,
            cr.duration_sec,
            cr.video_url
        FROM impressions i
        JOIN creatives cr ON cr.id = i.creative_id
        WHERE i.request_id = $1
    `, requestID)

	var (
		token      string
		creativeID uuid.UUID
		duration   int
		videoURL   string
	)

	if err := row.Scan(&token, &creativeID, &duration, &videoURL); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("findByRequestID: %w", err)
	}

	return &usecase.AdResponse{
		Token:      token,
		CreativeID: creativeID,
		Duration:   duration,
		VideoURL:   videoURL,
		ClickURL:   fmt.Sprintf("/ad/click/%s", token),
		ViewURL:    fmt.Sprintf("/ad/view/%s", token),
	}, nil
}

func nullOrString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
