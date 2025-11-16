package domain

import (
	"time"

	"github.com/google/uuid"
)

type PricingModel string

const (
	PricingCPM PricingModel = "CPM"
	PricingCPC PricingModel = "CPC"
)

type Campaign struct {
	ID           uuid.UUID
	Name         string
	PricingModel PricingModel
	BidMicros    int64

	BudgetTotal int64
	BudgetDaily int64

	SpentTotal int64
	SpentToday int64

	Active bool

	StartAt time.Time
	EndAt   *time.Time

	Targeting    Targeting
	FrequencyCap int
}
