package model

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"
)

const dateLayout = "2006-01-02"

// Date はPostgresのDATE型(時刻を持たない日付)に対応する値。
// time.Timeをそのまま使うとJSONに"2024-01-01T00:00:00Z"のような時刻が
// 混じって見づらいため、JSON表現を"YYYY-MM-DD"に固定するために用意する。
// runs.run_date・goals.target_dateの両方で使う想定。
type Date struct {
	time.Time
}

func NewDate(t time.Time) Date {
	return Date{Time: t}
}

func (d Date) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.Format(dateLayout) + `"`), nil
}

func (d *Date) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return fmt.Errorf("日付は YYYY-MM-DD 形式で指定してください: %w", err)
	}
	d.Time = t
	return nil
}

// Scan / Value は database/sql の Scanner/Valuer インターフェース。
// pgxはこのインターフェースを使ってDATE列とtime.Timeの相互変換を行う。
func (d *Date) Scan(value any) error {
	if value == nil {
		return nil
	}
	t, ok := value.(time.Time)
	if !ok {
		return fmt.Errorf("Dateにスキャンできない型です: %T", value)
	}
	d.Time = t
	return nil
}

func (d Date) Value() (driver.Value, error) {
	return d.Time, nil
}

func (d Date) IsZero() bool {
	return d.Time.IsZero()
}
