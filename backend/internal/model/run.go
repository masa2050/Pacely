package model

import "time"

// Run は docs/database.md 3.2 runs に対応する。
type Run struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	DistanceKm   float64   `json:"distance_km"`
	DurationSec  int       `json:"duration_sec"`
	PaceSecPerKm float64   `json:"pace_sec_per_km"`
	RPE          *int      `json:"rpe"`
	RunDate      Date      `json:"run_date"`
	CreatedAt    time.Time `json:"created_at"`
}
