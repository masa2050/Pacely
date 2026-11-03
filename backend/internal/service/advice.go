package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/masa2050/pacely/backend/internal/model"
	"github.com/masa2050/pacely/backend/internal/repository"
)

// docs/requirements.md 4.コールドスタート対応方針: 記録3件未満はAI呼び出しをせず
// ガイダンスのみ返す。
const minRunsForAdvice = 3

// adviceFreshWindow は「前回生成から24時間以内なら再生成しない」の閾値
// (docs/api.md 3.、docs/adr/004)。
const adviceFreshWindow = 24 * time.Hour

// maxAdviceGenerationsPerWindow は、adviceFreshWindow(24時間)の間に1ユーザーが生成できる
// 提案の上限(docs/adr/034)。目標を切り替えると24時間以内でも再生成するため、それを繰り返して
// AI APIの枠を使い切られないよう、回数で上限を保証する。通常の1回に、目標の作り直し
// (入力の間違いの修正など)2回分の余裕を足して3回とした。待ち時間(クールダウン)ではなく
// 回数にしたのは、1〜2回の作り直しなら待たずに新しい目標向けの提案を出せるようにするため。
const maxAdviceGenerationsPerWindow = 3

// adviceLeaseDuration は生成の権利(docs/adr/031)の有効期間。生成にかかる最悪の時間
// (天候API 10秒 + Gemini全体で90秒 = 約100秒)より長くする。短いと、生成の途中で
// 権利が切れて別のリクエストが取れてしまう。長すぎても、サーバーが生成中に落ちたときに
// 次の生成まで待たされる時間が延びるだけなので、余裕を持たせて3分とした。
const adviceLeaseDuration = 3 * time.Minute

// adviceGenerationTimeout は、権利を取ってから生成を終えるまでの上限。権利の期限より短くして、
// 生成が長引いても、権利が切れる前に必ず打ち切る(外部APIの最悪値の約100秒より長い)。
const adviceGenerationTimeout = adviceLeaseDuration - 30*time.Second

// recentRunsForAdvicePrompt はAIプロンプトに渡す「直近の記録」の件数。
// docs/adr/011で進捗計算に使った直近5件の考え方をそのまま再利用する
// (「直近の傾向」を渡すという目的が共通のため)。
const recentRunsForAdvicePrompt = recentRunsForProgress

// recentAdvicesForPrompt はAIプロンプトに渡す「前回までに提案したメニュー」の件数
// (docs/adr/021、8-2①)。直近5回連続で同じメニューに収束した実例があったため、
// 直近3回分を見せれば「前回と違う刺激を」という指示の根拠として十分と判断した。
// 件数を増やすほどプロンプトが長くなるため、まずは最小限から始める。
const recentAdvicesForPrompt = 3

// maxAdviceHistory は GET /advices が返す提案履歴の件数上限(docs/adr/020)。
// 履歴画面は「直近の傾向を振り返る」ための画面であり、それ以上遡る必要が出た
// 時点でページングを検討する。一覧に必要なのは日付・メニュー・評価だけだが、
// 本文を除いた一覧専用レスポンスを別に作るのはMVPに対して過剰なため、
// 既存エンドポイントに件数上限を付けるに留める。
const maxAdviceHistory = 50

// jst はプロンプトに渡す「今日の日付」(8-2③)をJST基準で計算するために使う
// (/code-review指摘)。本番はRailwayのalpine系イメージ(tzdata未導入)で動くため
// time.LoadLocation("Asia/Tokyo")は失敗しうる。日本はDSTが無く常にUTC+9固定のため、
// tzdataに依存しないFixedZoneで十分。
var jst = time.FixedZone("Asia/Tokyo", 9*60*60)

// AdviceStatus は GET /advices/latest のレスポンスがどの状態かを表す
// (docs/api.md 3.の生成判定フローに対応)。
type AdviceStatus string

const (
	// AdviceStatusReady はadvice(生成済みまたはキャッシュ)を返せる状態。
	AdviceStatusReady AdviceStatus = "ready"
	// AdviceStatusNeedsMoreRuns は記録がminRunsForAdvice未満の状態。
	AdviceStatusNeedsMoreRuns AdviceStatus = "needs_more_runs"
	// AdviceStatusNeedsActiveGoal は有効な目標が無い状態。
	AdviceStatusNeedsActiveGoal AdviceStatus = "needs_active_goal"
	// AdviceStatusGenerating は、別のリクエストがこのユーザーの提案を生成中の状態
	// (docs/adr/031)。adviceは含まない。FEは少し待ってから取り直す。
	AdviceStatusGenerating AdviceStatus = "generating"
)

// LatestAdviceResult は GET /advices/latest のレスポンス本体。
// AI呼び出しをスキップした場合(記録不足・目標未設定)もフロントが
// 分岐しやすいようStatusで明示する(docs/requirements.md 4.のガイダンス表示用)。
type LatestAdviceResult struct {
	Status          AdviceStatus  `json:"status"`
	Advice          *model.Advice `json:"advice,omitempty"`
	RunsCount       int           `json:"runs_count"`
	MinRunsRequired int           `json:"min_runs_required"`
}

type AdviceService struct {
	repo        *repository.AdviceRepository
	userService *UserService
	runRepo     *repository.RunRepository
	goalRepo    *repository.GoalRepository
	weather     WeatherClient
	ai          AdviceGenerator
}

func NewAdviceService(
	repo *repository.AdviceRepository,
	userService *UserService,
	runRepo *repository.RunRepository,
	goalRepo *repository.GoalRepository,
	weather WeatherClient,
	ai AdviceGenerator,
) *AdviceService {
	return &AdviceService{repo: repo, userService: userService, runRepo: runRepo, goalRepo: goalRepo, weather: weather, ai: ai}
}

// GetLatest は GET /advices/latest の生成判定・生成処理本体(docs/api.md 3.)。
// GETだが意図的に副作用(AI呼び出し・DB書き込み)を持つ設計であり、これは
// docs/adr/004で明記済みの意図的な判断(CLAUDE.mdのルール5)。
func (s *AdviceService) GetLatest(ctx context.Context, userID string) (*LatestAdviceResult, error) {
	runsCount, err := s.runRepo.CountByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if runsCount < minRunsForAdvice {
		return &LatestAdviceResult{Status: AdviceStatusNeedsMoreRuns, RunsCount: runsCount, MinRunsRequired: minRunsForAdvice}, nil
	}

	goal, err := s.goalRepo.GetActiveByUser(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return &LatestAdviceResult{Status: AdviceStatusNeedsActiveGoal, RunsCount: runsCount, MinRunsRequired: minRunsForAdvice}, nil
		}
		return nil, err
	}

	ready := func(advice *model.Advice) *LatestAdviceResult {
		return &LatestAdviceResult{Status: AdviceStatusReady, Advice: advice, RunsCount: runsCount, MinRunsRequired: minRunsForAdvice}
	}

	fresh, err := s.latestIfFresh(ctx, userID, goal.ID)
	if err != nil {
		return nil, err
	}
	if fresh != nil {
		return ready(fresh), nil
	}

	// ここから先は生成(10〜30秒)に入る。「確認」と「保存」の間に別のリクエストが割り込んで
	// 二重に生成しないよう、生成する権利を取れた1件だけが進む(docs/adr/031)。
	// 権利を取るUPDATEは、「他のリクエストが生成中」でも「usersの行が無い」でも0行になり、
	// 区別できない。行が無いのに generating を返し続けないよう、先に行の存在を保証する
	// (runsの保存前にusersの行を作るdocs/adr/008と同じ方針)。
	if _, err := s.userService.GetOrCreateMe(ctx, userID); err != nil {
		return nil, err
	}
	lease, err := s.repo.TryAcquireGenerationLease(ctx, userID, adviceLeaseDuration)
	if err != nil {
		return nil, err
	}
	if lease == nil {
		return &LatestAdviceResult{Status: AdviceStatusGenerating, RunsCount: runsCount, MinRunsRequired: minRunsForAdvice}, nil
	}
	defer s.releaseGenerationLease(ctx, lease)

	// 生成全体(DBの読み書き・天候API・AI API)が権利の期限に収まるよう、上限を付ける。
	// 収まらないと、生成の途中で権利が切れて別のリクエストが取れてしまい、二重生成になる。
	genCtx, cancel := context.WithTimeout(ctx, adviceGenerationTimeout)
	defer cancel()

	// 権利を取る間に、別のリクエストが生成を終えているかもしれない。上の確認は権利を取る前の
	// 古い状態なので、権利を持った状態でもう一度確認する(double-checked locking)。
	fresh, err = s.latestIfFresh(genCtx, userID, goal.ID)
	if err != nil {
		return nil, err
	}
	if fresh != nil {
		return ready(fresh), nil
	}

	// 24時間の上限回数に達していたら、目標が変わっていても生成せず、直前の提案を返す
	// (docs/adr/034)。前の目標向けの提案になりうるが、それは上限まで作り直した場合だけである。
	// 数えるのは権利を取った後にする。取る前に数えると、数えてから保存するまでの間に
	// 別のリクエストが割り込み、上限を超えて生成できてしまう(docs/adr/031と同じ競合)。
	generatedCount, err := s.repo.CountGeneratedSince(genCtx, userID, adviceFreshWindow)
	if err != nil {
		return nil, err
	}
	if generatedCount >= maxAdviceGenerationsPerWindow {
		latest, err := s.repo.GetLatestByUser(genCtx, userID)
		if err != nil {
			return nil, err
		}
		return ready(latest), nil
	}

	generated, err := s.generate(genCtx, userID, goal)
	if err != nil {
		return nil, err
	}
	return ready(generated), nil
}

// latestIfFresh は、現在の目標(goalID)向けで、前回の生成から24時間以内の最新の提案を返す。
// 次のいずれかの場合(=生成が必要)は(nil, nil)を返す。
//   - 提案が無い
//   - 24時間以上経っている
//   - 最新の提案のgoal_idが現在の目標と異なる、またはNULL(docs/adr/034)。
//     目標を切り替えた直後に、放棄済みの目標向けの提案を返さないため
//
// 権利を取る前の確認と取った後の再確認の両方から呼ばれるので、goalIDを必須の引数にして、
// どちらか一方だけ条件が抜ける事故を防いでいる。
func (s *AdviceService) latestIfFresh(ctx context.Context, userID, goalID string) (*model.Advice, error) {
	latest, err := s.repo.GetLatestByUser(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if time.Since(latest.GeneratedAt) >= adviceFreshWindow {
		return nil, nil
	}
	// goal_idがNULLの提案は、どの目標向けか確認できないので「違う」扱いにして再生成する。
	if latest.GoalID == nil || *latest.GoalID != goalID {
		return nil, nil
	}
	return latest, nil
}

// releaseGenerationLease は生成の権利を返す。成功・失敗のどちらでも呼ぶ。
// ブラウザが切断されてリクエストのctxが中断されても確実に返せるよう、ctxの中断を引き継がない
// 別のctxで実行する。返し損ねても期限(adviceLeaseDuration)が過ぎれば解放されるため、
// 失敗はログに残すだけで、ユーザーへのレスポンスは変えない。
func (s *AdviceService) releaseGenerationLease(ctx context.Context, lease *repository.GenerationLease) {
	releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := s.repo.ReleaseGenerationLease(releaseCtx, lease); err != nil {
		log.Printf("advice: failed to release generation lease: %v", err)
	}
}

// generate は天候取得→AI呼び出し→DB保存までを実行する(docs/api.md 3.のa〜d)。
func (s *AdviceService) generate(ctx context.Context, userID string, goal *model.Goal) (*model.Advice, error) {
	user, err := s.userService.GetOrCreateMe(ctx, userID)
	if err != nil {
		return nil, err
	}

	recentRuns, err := s.runRepo.ListRecentByUser(ctx, userID, recentRunsForAdvicePrompt)
	if err != nil {
		return nil, err
	}

	recentAdvices, err := s.repo.ListRecentByGoal(ctx, userID, goal.ID, recentAdvicesForPrompt)
	if err != nil {
		return nil, err
	}
	recentMenus := make([]model.NextMenu, len(recentAdvices))
	for i, a := range recentAdvices {
		recentMenus[i] = a.NextMenu
	}

	// regionが未設定の場合は天候APIを呼ばずに生成する(docs/adr/012)。
	// 「地域未設定」は入力エラーではなく想定内の状態であり、AI提案自体を
	// ブロックする理由にはならないため、天候情報なしのまま続行する。
	var weather model.WeatherContext
	if user.Region != nil && *user.Region != "" {
		weather, err = s.weather.CurrentWeather(ctx, *user.Region)
		if err != nil {
			return nil, err
		}
	}

	generation, err := s.ai.GenerateAdvice(ctx, AdvicePromptInput{
		GoalType:      goal.GoalType,
		TargetTimeSec: goal.TargetTimeSec,
		TargetDate:    goal.TargetDate,
		Today:         time.Now().In(jst),
		RecentRuns:    recentRuns,
		RecentMenus:   recentMenus,
		Weather:       weather,
	})
	if err != nil {
		return nil, err
	}

	advice := &model.Advice{
		UserID:         userID,
		GoalID:         &goal.ID,
		AdviceText:     generation.AdviceText,
		NextMenu:       generation.NextMenu,
		WeatherContext: weather,
	}
	return s.repo.Create(ctx, advice)
}

// List は GET /advices(過去の提案履歴一覧)。
func (s *AdviceService) List(ctx context.Context, userID string) ([]model.Advice, error) {
	return s.repo.ListByUser(ctx, userID, maxAdviceHistory)
}

// maxFeedbackCommentLen はフィードバックコメントの最大文字数。
// 自由入力を無制限に受け付けるとDB・将来のプロンプト投入時のコストが読めなくなるため、
// 「一言添える」用途に十分な長さで上限を設ける。
const maxFeedbackCommentLen = 500

// FeedbackInput は PUT /advices/{id}/feedback の入力値。
// IsHelpfulはポインタ: 未送信(nil)と false を区別して400を返すため。
type FeedbackInput struct {
	IsHelpful *bool
	Comment   *string
}

// SubmitFeedback は表示中のアドバイスへのフィードバックを保存する
// (フェーズ7-5、docs/adr/019)。
// 冪等な上書き更新であり、評価をやり直した場合は最後の内容だけが残る。
func (s *AdviceService) SubmitFeedback(ctx context.Context, userID, adviceID string, in FeedbackInput) (*model.Advice, error) {
	if in.IsHelpful == nil {
		return nil, fmt.Errorf("%w: 評価(役に立った/立たなかった)を選択してください", ErrInvalidInput)
	}

	comment := in.Comment
	if comment != nil {
		// 空白のみの入力は「コメント無し」として扱い、NULLで保存する。
		trimmed := strings.TrimSpace(*comment)
		if trimmed == "" {
			comment = nil
		} else {
			if utf8.RuneCountInString(trimmed) > maxFeedbackCommentLen {
				return nil, fmt.Errorf("%w: コメントは%d文字以内で入力してください", ErrInvalidInput, maxFeedbackCommentLen)
			}
			comment = &trimmed
		}
	}

	// 所有者チェックはservice層で行う(404と403を区別するため、runs/goalsと同じ方針)。
	existing, err := s.repo.GetByID(ctx, adviceID)
	if err != nil {
		return nil, err
	}
	if existing.UserID != userID {
		return nil, ErrForbidden
	}

	return s.repo.UpdateFeedback(ctx, adviceID, *in.IsHelpful, comment)
}
