package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/masa2050/pacely/backend/internal/model"
)

// WeatherClient は天候取得をインターフェースとして抽象化する。
// docs/architecture.md 7.: 外部API呼び出しはservice層に閉じ込め、
// 実装を差し替えられるようにしておく(プロバイダ変更・テスト時のfake差し替え用)。
type WeatherClient interface {
	CurrentWeather(ctx context.Context, region string) (model.WeatherContext, error)
}

const openWeatherMapBaseURL = "https://api.openweathermap.org/data/2.5/weather"

// OpenWeatherMapClient はOpenWeatherMap(docs/architecture.md 1.)の実装。
type OpenWeatherMapClient struct {
	apiKey     string
	httpClient *http.Client
}

func NewOpenWeatherMapClient(apiKey string) *OpenWeatherMapClient {
	return &OpenWeatherMapClient{apiKey: apiKey, httpClient: &http.Client{Timeout: 10 * time.Second}}
}

type openWeatherMapResponse struct {
	Weather []struct {
		Description string `json:"description"`
	} `json:"weather"`
	Main struct {
		Temp float64 `json:"temp"`
	} `json:"main"`
}

// CurrentWeather はusers.region(都道府県/市の文字列、docs/database.md 3.1)を
// そのままOpenWeatherMapの都市名検索(q=)に渡す。region自体の緯度経度変換や
// 表記ゆれの吸収はMVPスコープ外とする(docs/requirements.md 6.)。
func (c *OpenWeatherMapClient) CurrentWeather(ctx context.Context, region string) (model.WeatherContext, error) {
	q := url.Values{}
	q.Set("q", region)
	q.Set("appid", c.apiKey)
	q.Set("units", "metric")
	q.Set("lang", "ja")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, openWeatherMapBaseURL+"?"+q.Encode(), nil)
	if err != nil {
		return model.WeatherContext{}, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return model.WeatherContext{}, fmt.Errorf("天候API呼び出しに失敗: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return model.WeatherContext{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return model.WeatherContext{}, fmt.Errorf("天候APIがエラーを返しました(status %d): %s", resp.StatusCode, string(body))
	}

	var parsed openWeatherMapResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return model.WeatherContext{}, fmt.Errorf("天候APIのレスポンス解析に失敗: %w", err)
	}

	description := ""
	if len(parsed.Weather) > 0 {
		description = parsed.Weather[0].Description
	}
	return model.WeatherContext{
		HasWeather:  true,
		Description: description,
		TempC:       parsed.Main.Temp,
	}, nil
}
