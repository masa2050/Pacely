package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// Advice は docs/database.md 3.4 advices に対応する。
type Advice struct {
	ID             string         `json:"id"`
	UserID         string         `json:"user_id"`
	GoalID         *string        `json:"goal_id"`
	AdviceText     string         `json:"advice_text"`
	NextMenu       NextMenu       `json:"next_menu"`
	WeatherContext WeatherContext `json:"weather_context"`
	GeneratedAt    time.Time      `json:"generated_at"`
}

// NextMenu は次回練習メニュー(docs/database.md 3.4 next_menu)。
// jsonb列に対応させるため、Scan/ValueでJSON⇔Goの相互変換を行う。
type NextMenu struct {
	DistanceKm   float64 `json:"distance_km"`
	PaceSecPerKm float64 `json:"pace_sec_per_km"`
	Note         string  `json:"note"`
}

func (n *NextMenu) Scan(value any) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("NextMenuにスキャンできない型です: %T", value)
	}
	return json.Unmarshal(b, n)
}

func (n NextMenu) Value() (driver.Value, error) {
	return json.Marshal(n)
}

// WeatherContext は生成時に参照した天候情報(docs/database.md 3.4 weather_context)。
// users.regionが未設定のユーザーは天候APIを呼ばずに生成するため、
// その場合はHasWeather=falseのゼロ値がweather_context=NULLとして保存される
// (docs/adr/012参照)。
type WeatherContext struct {
	HasWeather  bool    `json:"has_weather"`
	Description string  `json:"description,omitempty"`
	TempC       float64 `json:"temp_c,omitempty"`
}

func (w *WeatherContext) Scan(value any) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("WeatherContextにスキャンできない型です: %T", value)
	}
	return json.Unmarshal(b, w)
}

// Value はHasWeather=falseの場合、jsonbとしてはNULLを保存する
// (weather_contextカラムはnullable、docs/database.md 3.4)。
func (w WeatherContext) Value() (driver.Value, error) {
	if !w.HasWeather {
		return nil, nil
	}
	return json.Marshal(w)
}
