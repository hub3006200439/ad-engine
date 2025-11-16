package domain

import (
	"time"

	"github.com/google/uuid"
)

type Impression struct {
	ID         uuid.UUID
	Token      string // внешний токен
	CampaignID uuid.UUID
	CreativeID uuid.UUID
	UserID     uuid.UUID
	SessionID  uuid.UUID
	CreatedAt  time.Time
}

type Click struct {
	ID           uuid.UUID
	ImpressionID uuid.UUID
	Token        string
	CreatedAt    time.Time
}
