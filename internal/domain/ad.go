package domain

type AdRequest struct {
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	Lang      string `json:"lang"`
	Country   string `json:"country"`
	Category  string `json:"category"`
	Placement string `json:"placement"`
}

type AdResponse struct {
	Token      string `json:"token"`
	CreativeID int64  `json:"creative_id"`
	Duration   int    `json:"duration"`
	VideoURL   string `json:"video_url"`
	ClickURL   string `json:"click_url"`
}
