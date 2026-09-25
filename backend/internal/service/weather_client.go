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

// prefectureToCity は users.region に保存された都道府県名を、OpenWeatherMapの
// 都市名検索(q=)に渡す文字列へ変換する表(docs/adr/018)。
//
// 都道府県名のローマ字表記はほとんどそのまま引けるが、以下2県は同名の別地域に
// 当たってしまうことを実測で確認したため、県庁所在地を指定している:
//   - Ibaraki  → 大阪府茨木市(34.8N 135.6E)にヒットするため Mito(水戸市)を使う
//   - Ishikawa → 沖縄県うるま市石川(26.4N 127.8E)にヒットするため Kanazawa(金沢市)を使う
//
// この変換表をWeatherClientインターフェースではなくOpenWeatherMap実装側に置いているのは、
// 「どの検索キーワードで引けるか」がプロバイダ固有の事情であり、プロバイダを差し替えたら
// 変換内容も変わるため(docs/architecture.md 7.)。
var prefectureToCity = map[string]string{
	"北海道":  "Hokkaido",
	"青森県":  "Aomori",
	"岩手県":  "Iwate",
	"宮城県":  "Miyagi",
	"秋田県":  "Akita",
	"山形県":  "Yamagata",
	"福島県":  "Fukushima",
	"茨城県":  "Mito",
	"栃木県":  "Tochigi",
	"群馬県":  "Gunma",
	"埼玉県":  "Saitama",
	"千葉県":  "Chiba",
	"東京都":  "Tokyo",
	"神奈川県": "Kanagawa",
	"新潟県":  "Niigata",
	"富山県":  "Toyama",
	"石川県":  "Kanazawa",
	"福井県":  "Fukui",
	"山梨県":  "Yamanashi",
	"長野県":  "Nagano",
	"岐阜県":  "Gifu",
	"静岡県":  "Shizuoka",
	"愛知県":  "Aichi",
	"三重県":  "Mie",
	"滋賀県":  "Shiga",
	"京都府":  "Kyoto",
	"大阪府":  "Osaka",
	"兵庫県":  "Hyogo",
	"奈良県":  "Nara",
	"和歌山県": "Wakayama",
	"鳥取県":  "Tottori",
	"島根県":  "Shimane",
	"岡山県":  "Okayama",
	"広島県":  "Hiroshima",
	"山口県":  "Yamaguchi",
	"徳島県":  "Tokushima",
	"香川県":  "Kagawa",
	"愛媛県":  "Ehime",
	"高知県":  "Kochi",
	"福岡県":  "Fukuoka",
	"佐賀県":  "Saga",
	"長崎県":  "Nagasaki",
	"熊本県":  "Kumamoto",
	"大分県":  "Oita",
	"宮崎県":  "Miyazaki",
	"鹿児島県": "Kagoshima",
	"沖縄県":  "Okinawa",
}

// buildQuery は region をOpenWeatherMapの q= に渡す値に変換する。
// 都道府県名なら国コード付きの都市名("Tokyo,JP")にして、同名の海外都市に
// ヒットするのを防ぐ。変換表に無い値は、フェーズ7-3以前に自由入力で保存された
// ローマ字表記(例: "Tokyo")とみなしてそのまま渡す(後方互換)。
func buildQuery(region string) string {
	if city, ok := prefectureToCity[region]; ok {
		return city + ",JP"
	}
	return region
}

type openWeatherMapResponse struct {
	Weather []struct {
		Description string `json:"description"`
	} `json:"weather"`
	Main struct {
		Temp float64 `json:"temp"`
	} `json:"main"`
}

// CurrentWeather はusers.region(都道府県名、docs/database.md 3.1)をOpenWeatherMapの
// 都市名検索(q=)に渡して現在の天候を取得する。都道府県名→検索キーワードの変換は
// buildQueryが行う。緯度経度への変換(Geocoding API)はMVPスコープ外とする
// (docs/requirements.md 6.)。
func (c *OpenWeatherMapClient) CurrentWeather(ctx context.Context, region string) (model.WeatherContext, error) {
	q := url.Values{}
	q.Set("q", buildQuery(region))
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
