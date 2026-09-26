package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/masa2050/pacely/backend/internal/model"
)

// ErrExternalAPI はAI API・天候APIなど外部API呼び出しが失敗したことを表すsentinel error。
// docs/implementation-plan.md 8-1: これらのエラーの生メッセージ(*url.Errorが含む
// クエリ文字列込みのURL、レスポンス本文)にはAPIキーが含まれうるため、handler層で
// このエラーだと判定した場合は定型メッセージのみを返す。詳細はlog.Printfでサーバー側にのみ残す。
var ErrExternalAPI = errors.New("external api failed")

// AdvicePromptInput はAIへのプロンプトに必要な材料をまとめたもの。
// プロバイダ(Gemini/Claude等)に依存しない形にしておくことで、
// docs/adr/002の通り本番前にプロバイダを差し替えてもservice層の
// 呼び出し側(AdviceService)には影響しない。
type AdvicePromptInput struct {
	GoalType      string
	TargetTimeSec int
	TargetDate    model.Date
	// Today は生成時点の日付。docs/adr/021・8-2③: レースまでの残日数をAIに
	// 判断させるための基準日として渡す(buildPrompt側でTargetDateとの差分を計算する)。
	Today      time.Time
	RecentRuns []model.Run
	// RecentMenus は直近に提案したnext_menu(新しい順)。空の場合は初回提案として扱う。
	// docs/adr/021・8-2①: 「前回と違う刺激を」とAIに明示するために渡す。
	RecentMenus []model.NextMenu
	Weather     model.WeatherContext
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

	// recentMenusDescは「前回までに何を提案したか」をAIに伝える(docs/adr/021、8-2①)。
	// これが無いとAIは自分の過去の提案を知らず、毎回ゼロから最適解を考えて同じ答えに
	// 収束してしまう(距離5/8/10km・340秒/kmのペース走が5回連続で提案された実例あり)。
	recentMenusDesc := "前回までの提案なし(初回の提案)"
	if len(in.RecentMenus) > 0 {
		var b strings.Builder
		for i, m := range in.RecentMenus {
			// menu_typeが空なのは8-2②導入前に生成された古い提案(後方互換)。
			// ここで「種別不明」のような代替ラベルを本物の種別と同じ位置に置くと、
			// Geminiがそれを選択肢の一つと誤解してmenu_typeにそのまま返してくる
			// (本番で実際に発生)。種別が無い行では種別のスロット自体を空にする。
			if m.MenuType != "" {
				fmt.Fprintf(&b, "- %d回前: %s %.1fkm / %.0f秒/km", i+1, m.MenuType, m.DistanceKm, m.PaceSecPerKm)
			} else {
				fmt.Fprintf(&b, "- %d回前: %.1fkm / %.0f秒/km", i+1, m.DistanceKm, m.PaceSecPerKm)
			}
			if m.Note != "" {
				fmt.Fprintf(&b, " (%s)", m.Note)
			}
			b.WriteString("\n")
		}
		recentMenusDesc = b.String()
	}

	// daysUntilRaceは日付部分のみで日数差を計算する(時刻を含めるとタイムゾーン・
	// 実行時刻次第で±1日ずれるため)。docs/adr/021・8-2③: 練習の位置づけ
	// (まだ余裕がある/直前で調整に入るべき等)をAIが判断する材料として渡す。
	today := time.Date(in.Today.Year(), in.Today.Month(), in.Today.Day(), 0, 0, 0, 0, time.UTC)
	targetDate := time.Date(in.TargetDate.Year(), in.TargetDate.Month(), in.TargetDate.Day(), 0, 0, 0, 0, time.UTC)
	daysUntilRace := int(targetDate.Sub(today).Hours() / 24)

	// goals.status=activeのままtarget_dateが過去日になっているケース(レース後も
	// 目標を編集していないユーザー)では、「残り-5日」という不自然な表現になるため
	// 分けて表記する(/code-review指摘)。
	daysUntilRaceDesc := fmt.Sprintf("残り%d日", daysUntilRace)
	if daysUntilRace < 0 {
		daysUntilRaceDesc = fmt.Sprintf("目標達成予定日を%d日過ぎています", -daysUntilRace)
	}

	return fmt.Sprintf(`あなたはランニングコーチです。以下のランナーの情報をもとに、次の練習に向けたアドバイスと具体的な練習メニューを提案してください。

## 今日の日付
%s

## 目標
種目: %s
目標タイム: %d秒
目標達成予定日: %s(%s)

## 直近の記録
%s

## 前回までに提案した練習メニュー(新しい順)
%s
同じ種別・同じ距離・同じペースの練習を連続で提案しないでください。前回までの提案を踏まえ、
今回は意図的に違う刺激(練習の種別を変える、距離やペースを変える、負荷を上げる/下げる等)を
与える練習を提案してください。練習には休養日も含まれます。連日ハードな練習を提案し続けず、
必要に応じて休養やジョグ(軽い回復走)も選択肢に入れてください。

## 現在の天候
%s

## 出力形式
以下のJSON形式のみで出力してください。説明文やコードブロックの記号(%s)は不要です。
{
  "advice_text": "アドバイス本文(日本語、200字程度)",
  "next_menu": {
    "menu_type": 次回練習の種別。上記「前回までに提案した練習メニュー」の表記に関わらず、必ず次のいずれか1つを完全一致で使うこと: %s,
    "distance_km": 次回練習の距離(数値、km)。menu_typeが"%s"の場合は0,
    "pace_sec_per_km": 次回練習の目標ペース(数値、秒/km)。menu_typeが"%s"の場合は0,
    "note": "練習メニューの補足(日本語、1〜2文)",
    "segments": menu_typeが"%s"の場合のみ、2〜4個の区間を配列で出力する。各区間は
      {"reps": 1, "distance_km": 区間の距離(数値), "pace_sec_per_km": 区間の目標ペース(数値), "rest_sec": 0}。
      区間は距離が短い順・ペースが徐々に速くなる順に並べ、distance_kmの合計は上のdistance_kmと一致させること。
      menu_typeが"%s"以外の場合はsegmentsキー自体を出力しないこと
  }
}`,
		today.Format("2006-01-02"),
		in.GoalType, in.TargetTimeSec, in.TargetDate.Format("2006-01-02"), daysUntilRaceDesc,
		runsDesc.String(), recentMenusDesc, weatherDesc, "```",
		`"`+strings.Join(model.ValidMenuTypes, `", "`)+`"`, model.MenuTypeRest, model.MenuTypeRest,
		model.MenuTypeBuildUp, model.MenuTypeBuildUp)
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
	// 無料枠はピーク時に他の利用者と処理能力を共有するため、応答が遅延することがある
	// (2026-09-22 本番で30秒ちょうどでのタイムアウトを確認、devlog参照)。無料枠を使い続ける前提の
	// 暫定対応として60秒に延長する。根本対応ではなく、頻発するようなら有料ティアへの切り替えを検討する。
	return &GeminiClient{apiKey: apiKey, httpClient: &http.Client{Timeout: 60 * time.Second}}
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
		MenuType     string  `json:"menu_type"`
		DistanceKm   float64 `json:"distance_km"`
		PaceSecPerKm float64 `json:"pace_sec_per_km"`
		Note         string  `json:"note"`
		// Segmentsはmenu_typeが"ビルドアップ走"の場合のみ入る想定(docs/adr/022)。
		Segments []model.MenuSegment `json:"segments"`
	} `json:"next_menu"`
}

func (c *GeminiClient) GenerateAdvice(ctx context.Context, in AdvicePromptInput) (AdviceGeneration, error) {
	reqBody := geminiRequest{
		Contents:         []geminiContent{{Parts: []geminiPart{{Text: buildPrompt(in)}}}},
		GenerationConfig: geminiGenerationConfig{ResponseMimeType: "application/json"},
	}
	b, err := json.Marshal(reqBody)
	if err != nil {
		// reqBodyは文字列のみで現状Marshal失敗は起きないが、8-1の方針(外部API絡みの
		// エラーはerr.Error()をそのまま返さない)に合わせて他の分岐と統一しておく。
		log.Printf("advice: gemini request marshal failed: %v", err)
		return AdviceGeneration{}, fmt.Errorf("%w: AIリクエストの組み立てに失敗しました", ErrExternalAPI)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, geminiEndpoint+"?key="+c.apiKey, bytes.NewReader(b))
	if err != nil {
		// errはAPIキー込みのURLを含みうるためログのみ。
		log.Printf("advice: gemini api request build failed: %v", err)
		return AdviceGeneration{}, fmt.Errorf("%w: AIリクエストの作成に失敗しました", ErrExternalAPI)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		// errは*url.Errorで、クエリ文字列(?key=<APIキー>)込みのリクエストURLを含むため
		// ログにのみ残し、呼び出し元にはErrExternalAPIの定型メッセージだけを返す。
		log.Printf("advice: gemini api call failed: %v", err)
		return AdviceGeneration{}, fmt.Errorf("%w: AI API呼び出しに失敗しました", ErrExternalAPI)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("advice: gemini api response read failed: %v", err)
		return AdviceGeneration{}, fmt.Errorf("%w: AI APIのレスポンス取得に失敗しました", ErrExternalAPI)
	}
	if resp.StatusCode != http.StatusOK {
		// respBodyはGoogle側の生JSON(429時のエラー詳細等)なのでログのみ。
		log.Printf("advice: gemini api returned status %d: %s", resp.StatusCode, string(respBody))
		return AdviceGeneration{}, fmt.Errorf("%w: AI APIがエラーを返しました(status %d)", ErrExternalAPI, resp.StatusCode)
	}

	var gr geminiResponse
	if err := json.Unmarshal(respBody, &gr); err != nil {
		log.Printf("advice: gemini api response parse failed: %v", err)
		return AdviceGeneration{}, fmt.Errorf("%w: AI APIのレスポンス解析に失敗しました", ErrExternalAPI)
	}
	if len(gr.Candidates) == 0 || len(gr.Candidates[0].Content.Parts) == 0 {
		log.Printf("advice: gemini api response has no candidates: %s", string(respBody))
		return AdviceGeneration{}, fmt.Errorf("%w: AI APIのレスポンスに候補がありません", ErrExternalAPI)
	}

	var parsed adviceJSON
	if err := json.Unmarshal([]byte(gr.Candidates[0].Content.Parts[0].Text), &parsed); err != nil {
		log.Printf("advice: gemini output json parse failed: %v (raw: %s)", err, gr.Candidates[0].Content.Parts[0].Text)
		return AdviceGeneration{}, fmt.Errorf("%w: AI出力のJSON解析に失敗しました", ErrExternalAPI)
	}
	// menu_typeはプロンプトで列挙値を指示しているが、AIが指示通りに返す保証はない
	// (jsonbは値を制約しないため実行時エラーにはできない、docs/database.md 5.)。
	// 提案自体は止めず、想定外の値だった場合だけログに残して後から傾向を追えるようにする。
	if !model.IsValidMenuType(parsed.NextMenu.MenuType) {
		log.Printf("advice: gemini returned unexpected menu_type: %q", parsed.NextMenu.MenuType)
	}
	// segmentsもmenu_type同様、AIが指示(docs/adr/022)通りに返す保証はない。
	// ビルドアップ走なのにsegmentsが無い/他の種別なのにsegmentsがある/距離の合計が
	// distance_kmと合わない、のいずれも生成自体は止めずログにのみ残す。
	switch {
	case parsed.NextMenu.MenuType == model.MenuTypeBuildUp && len(parsed.NextMenu.Segments) == 0:
		log.Printf("advice: gemini menu_type=%s but segments is empty", model.MenuTypeBuildUp)
	case parsed.NextMenu.MenuType != model.MenuTypeBuildUp && len(parsed.NextMenu.Segments) > 0:
		log.Printf("advice: gemini returned segments for non-buildup menu_type %q", parsed.NextMenu.MenuType)
	}
	if len(parsed.NextMenu.Segments) > 0 {
		var sum float64
		for _, seg := range parsed.NextMenu.Segments {
			reps := seg.Reps
			if reps < 1 {
				reps = 1
			}
			sum += seg.DistanceKm * reps
		}
		if diff := sum - parsed.NextMenu.DistanceKm; diff > 1 || diff < -1 {
			log.Printf("advice: gemini segments distance sum (%.1f) does not match distance_km (%.1f)",
				sum, parsed.NextMenu.DistanceKm)
		}
	}

	return AdviceGeneration{
		AdviceText: parsed.AdviceText,
		NextMenu: model.NextMenu{
			MenuType:     parsed.NextMenu.MenuType,
			DistanceKm:   parsed.NextMenu.DistanceKm,
			PaceSecPerKm: parsed.NextMenu.PaceSecPerKm,
			Note:         parsed.NextMenu.Note,
			Segments:     parsed.NextMenu.Segments,
		},
	}, nil
}
