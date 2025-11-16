package domain

import "github.com/google/uuid"

type StatsOverview struct {
	CampaignID  uuid.UUID `json:"campaign_id"`
	Impressions int64     `json:"impressions"`
	Clicks      int64     `json:"clicks"`
	CTR         float64   `json:"ctr"`
	SpendMicros int64     `json:"spend_micros"`
}
