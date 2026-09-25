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

	// フィードバック(フェーズ7-5、docs/adr/019)。
	// 3つともnil = 未評価。1:1の関係なので別テーブルにせずadvicesのカラムとして持つ。
	IsHelpful       *bool      `json:"is_helpful"`
	FeedbackComment *string    `json:"feedback_comment"`
	FeedbackAt      *time.Time `json:"feedback_at"`
}

// MenuType の許容値(docs/implementation-plan.md 8-2②、docs/adr/021)。
// distance_km/pace_sec_per_km だけでは「一定ペースで何km走るか」しか表現できず
// 構造的にペース走へ収束していたため、練習の種別を明示的に持たせる。
const (
	MenuTypeJog      = "ジョグ"
	MenuTypePace     = "ペース走"
	MenuTypeInterval = "インターバル"
	MenuTypeLong     = "ロング走"
	MenuTypeRest     = "休養"
)

// ValidMenuTypes はAI出力の検証・FE表示の両方から参照する順序付きの一覧。
var ValidMenuTypes = []string{MenuTypeJog, MenuTypePace, MenuTypeInterval, MenuTypeLong, MenuTypeRest}

// IsValidMenuType はAIが指示通りの列挙値を返したかを確認する。
// 想定外の値でもエラーにはせず(jsonbは値を制約しない、docs/database.md 5.)、
// 呼び出し側でログに残すためだけに使う。
func IsValidMenuType(s string) bool {
	for _, v := range ValidMenuTypes {
		if s == v {
			return true
		}
	}
	return false
}

// NextMenu は次回練習メニュー(docs/database.md 3.4 next_menu)。
// jsonb列に対応させるため、Scan/ValueでJSON⇔Goの相互変換を行う。
type NextMenu struct {
	MenuType     string  `json:"menu_type"`
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
