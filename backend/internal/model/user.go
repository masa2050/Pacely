package model

import "time"

// User は docs/database.md 3.1 users に対応する。
// id は Supabase Auth の user id (JWTのsub) と同一。
type User struct {
	ID        string    `json:"id"`
	Region    *string   `json:"region"`
	Username  *string   `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}
