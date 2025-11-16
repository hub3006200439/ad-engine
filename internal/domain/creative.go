package domain

import "github.com/google/uuid"

type Creative struct {
	ID          uuid.UUID
	CampaignID  uuid.UUID
	DurationSec int
	VideoURL    string
	LandingURL  string
	Lang        string
	Placement   string
	Category    string
}
