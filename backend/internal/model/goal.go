package model

import "time"

// docs/database.md 3.3 goals.status で使う値。
const (
	GoalStatusActive    = "active"
	GoalStatusAchieved  = "achieved"
	GoalStatusAbandoned = "abandoned"
)

// Goal は docs/database.md 3.3 goals に対応する。
type Goal struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	GoalType      string    `json:"goal_type"`
	TargetTimeSec int       `json:"target_time_sec"`
	TargetDate    Date      `json:"target_date"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}
