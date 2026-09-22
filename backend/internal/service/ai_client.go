package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/masa2050/pacely/backend/internal/model"
)

// AdvicePromptInput はAIへのプロンプトに必要な材料をまとめたもの。
// プロバイダ(Gemini/Claude等)に依存しない形にしておくことで、
// docs/adr/002の通り本番前にプロバイダを差し替えてもservice層の
// 呼び出し側(AdviceService)には影響しない。
type AdvicePromptInput struct {
	GoalType      string
	TargetTimeSec int
	TargetDate    model.Date
	RecentRuns    []model.Run
	Weather       model.WeatherContext
}

// AdviceGeneration はAI生成結果(docs/database.md 3.4のadvice_text/next_menuに対応)。
type AdviceGeneration struct {
	AdviceText string
	NextMenu   model.NextMenu
}

// AdviceGenerator はAI提案生成をインターフェースとして抽象化する。
// docs/adr/002: 開発段階ではGemini API(無料枠)を使うが、本番前にClaude/OpenAIと
// 比較して差し替える可能性があるため、AdviceServiceはこのインターフェースにのみ依存する。
type AdviceGenerator interface {
	GenerateAdvice(ctx context.Context, in AdvicePromptInput) (AdviceGeneration, error)
}

// buildPrompt はGoal・直近の記録・天候情報からAIへのプロンプト文字列を組み立てる。
// 出力形式をJSONに固定するのは、advice_text/next_menuをそのままDBのjsonb列
// (docs/database.md 3.4)に保存できるようにするため。
func buildPrompt(in AdvicePromptInput) string {
	var runsDesc strings.Builder
	if len(in.RecentRuns) == 0 {
		runsDesc.WriteString("直近の記録なし")
	}
	for _, r := range in.RecentRuns {
		fmt.Fprintf(&runsDesc, "- %s: %.1fkm / %d秒 (ペース%.0f秒/km)",
			r.RunDate.Format("2006-01-02"), r.DistanceKm, r.DurationSec, r.PaceSecPerKm)
		if r.RPE != nil {
			fmt.Fprintf(&runsDesc, " / 体感的きつさRPE %d/10", *r.RPE)
		}
		runsDesc.WriteString("\n")
	}

	weatherDesc := "天候情報なし(地域未設定)"
	if in.Weather.HasWeather {
		weatherDesc = fmt.Sprintf("%s / 気温%.1f℃", in.Weather.Description, in.Weather.TempC)
	}

	return fmt.Sprintf(`あなたはランニングコーチです。以下のランナーの情報をもとに、次の練習に向けたアドバイスと具体的な練習メニューを提案してください。

## 目標
種目: %s
目標タイム: %d秒
目標達成予定日: %s

## 直近の記録
%s

## 現在の天候
%s

## 出力形式
以下のJSON形式のみで出力してください。説明文やコードブロックの記号(%s)は不要です。
{
  "advice_text": "アドバイス本文(日本語、200字程度)",
  "next_menu": {
    "distance_km": 次回練習の距離(数値、km),
    "pace_sec_per_km": 次回練習の目標ペース(数値、秒/km),
    "note": "練習メニューの補足(日本語、1〜2文)"
  }
}`,
		in.GoalType, in.TargetTimeSec, in.TargetDate.Format("2006-01-02"),
		runsDesc.String(), weatherDesc, "```")
}

const (
	// gemini-2.0-flashはGoogle側で廃止された(2026年時点、404 NOT_FOUND)ため
	// gemini-3.6-flashに変更。将来またモデルが廃止された場合はここだけ直せばよい。
	geminiModel    = "gemini-3.6-flash"
	geminiEndpoint = "https://generativelanguage.googleapis.com/v1beta/models/" + geminiModel + ":generateContent"
)

// GeminiClient はGoogle Gemini API(docs/adr/002)の実装。
// 専用SDKは導入せず、REST呼び出しのみで完結させる(go.mod依存を最小限に保つ、
// docs/architecture.md 過剰設計をしない方針)。
type GeminiClient struct {
	apiKey     string
	httpClient *http.Client
}

func NewGeminiClient(apiKey string) *GeminiClient {
	return &GeminiClient{apiKey: apiKey, httpClient: &http.Client{Timeout: 30 * time.Second}}
}

type geminiRequest struct {
	Contents         []geminiContent        `json:"contents"`
	GenerationConfig geminiGenerationConfig `json:"generationConfig"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerationConfig struct {
	// responseMimeTypeをapplication/jsonに固定し、Gemini側にJSON以外の
	// 出力(前置きの説明文など)を防いでもらう。
	ResponseMimeType string `json:"responseMimeType"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

// adviceJSON はGeminiに出力させるJSONの構造(buildPromptの出力形式と対応)。
type adviceJSON struct {
	AdviceText string `json:"advice_text"`
	NextMenu   struct {
		DistanceKm   float64 `json:"distance_km"`
		PaceSecPerKm float64 `json:"pace_sec_per_km"`
		Note         string  `json:"note"`
	} `json:"next_menu"`
}

func (c *GeminiClient) GenerateAdvice(ctx context.Context, in AdvicePromptInput) (AdviceGeneration, error) {
	reqBody := geminiRequest{
		Contents:         []geminiContent{{Parts: []geminiPart{{Text: buildPrompt(in)}}}},
		GenerationConfig: geminiGenerationConfig{ResponseMimeType: "application/json"},
	}
	b, err := json.Marshal(reqBody)
	if err != nil {
		return AdviceGeneration{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, geminiEndpoint+"?key="+c.apiKey, bytes.NewReader(b))
	if err != nil {
		return AdviceGeneration{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return AdviceGeneration{}, fmt.Errorf("AI API呼び出しに失敗: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return AdviceGeneration{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return AdviceGeneration{}, fmt.Errorf("AI APIがエラーを返しました(status %d): %s", resp.StatusCode, string(respBody))
	}

	var gr geminiResponse
	if err := json.Unmarshal(respBody, &gr); err != nil {
		return AdviceGeneration{}, fmt.Errorf("AI APIのレスポンス解析に失敗: %w", err)
	}
	if len(gr.Candidates) == 0 || len(gr.Candidates[0].Content.Parts) == 0 {
		return AdviceGeneration{}, fmt.Errorf("AI APIのレスポンスに候補がありません")
	}

	var parsed adviceJSON
	if err := json.Unmarshal([]byte(gr.Candidates[0].Content.Parts[0].Text), &parsed); err != nil {
		return AdviceGeneration{}, fmt.Errorf("AI出力のJSON解析に失敗: %w", err)
	}

	return AdviceGeneration{
		AdviceText: parsed.AdviceText,
		NextMenu: model.NextMenu{
			DistanceKm:   parsed.NextMenu.DistanceKm,
			PaceSecPerKm: parsed.NextMenu.PaceSecPerKm,
			Note:         parsed.NextMenu.Note,
		},
	}, nil
}
