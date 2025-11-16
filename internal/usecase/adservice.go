package usecase

import (
	"context"

	"github.com/google/uuid"
)

// AdRequest — DTO для запроса показа (HTTP → usecase → repo).
type AdRequest struct {
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	Lang      string `json:"lang"`
	Country   string `json:"country"`
	Category  string `json:"category"`
	Placement string `json:"placement"`
	// request_id — опциональный идемпотентный ключ для /ad/request
	RequestID string `json:"request_id,omitempty"`
}

// AdResponse — DTO ответа с креативом.
type AdResponse struct {
	Token      string    `json:"token"`
	CreativeID uuid.UUID `json:"creative_id"`
	Duration   int       `json:"duration"`
	VideoURL   string    `json:"video_url"`
	ClickURL   string    `json:"click_url"`
	ViewURL    string    `json:"view_url"`
}

// AdRepository — контракт, который реализует слой хранения (Postgres).
type AdRepository interface {
	RequestAd(ctx context.Context, req AdRequest) (*AdResponse, error)
	TrackClick(ctx context.Context, token string) (string, error)
	TrackView(ctx context.Context, token string) error
}

// AdService — usecase-слой поверх репозитория рекламы.
type AdService struct {
	repo AdRepository
	freq *FrequencyService // пока не используется напрямую, но инжектится для возможных расширений
}

// NewAdService создаёт новый сервис показа рекламы.
func NewAdService(repo AdRepository, freq *FrequencyService) *AdService {
	return &AdService{
		repo: repo,
		freq: freq,
	}
}

// RequestAd — публичный usecase для подбора объявления.
func (s *AdService) RequestAd(ctx context.Context, req AdRequest) (*AdResponse, error) {
	return s.repo.RequestAd(ctx, req)
}

// TrackClick — фиксирует клик и, при CPC-модели, инициирует списание бюджета.
// Идемпотентность реализована на уровне репозитория (UNIQUE(impression_id)).
func (s *AdService) TrackClick(ctx context.Context, token string) (string, error) {
	return s.repo.TrackClick(ctx, token)
}

// TrackView — фиксирует факт просмотра (view) по impression token.
// Идемпотентность также на уровне репозитория (UNIQUE(impression_id)).
func (s *AdService) TrackView(ctx context.Context, token string) error {
	return s.repo.TrackView(ctx, token)
}
