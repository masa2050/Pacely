package service

import (
	"context"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/masa2050/pacely/backend/internal/model"
	"github.com/masa2050/pacely/backend/internal/repository"
)

// countingGenerator はAI呼び出しの回数を数えるダミー。実際のGeminiは呼ばない。
// delayは「AIの応答待ち(本物は10〜30秒)」の代わりで、この間に別のリクエストが
// 割り込める状態を作る。
type countingGenerator struct {
	calls atomic.Int32
	delay time.Duration
}

func (g *countingGenerator) GenerateAdvice(ctx context.Context, _ AdvicePromptInput) (AdviceGeneration, error) {
	g.calls.Add(1)
	select {
	case <-time.After(g.delay):
	case <-ctx.Done():
		return AdviceGeneration{}, ctx.Err()
	}
	return AdviceGeneration{
		AdviceText: "テスト用の提案",
		NextMenu:   model.NextMenu{MenuType: model.MenuTypeRest},
	}, nil
}

type noWeather struct{}

func (noWeather) CurrentWeather(context.Context, string) (model.WeatherContext, error) {
	return model.WeatherContext{}, nil
}

// newTestPool はローカルDBへの接続を返す。TEST_DATABASE_URLが無ければスキップする
// (CIにはDBが無いため)。本番DBに誤って繋がないよう、ホストがlocalhost以外なら失敗させる。
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL が未設定のためスキップ(ローカルDBが必要)")
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("TEST_DATABASE_URL を解釈できません: %v", err)
	}
	if h := u.Hostname(); h != "localhost" && h != "127.0.0.1" {
		t.Fatalf("TEST_DATABASE_URL のホストが %q です。テストはローカルDBにだけ接続できます", h)
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("DB接続に失敗: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// 同じユーザーの GetLatest が同時に何度呼ばれても、AIの呼び出しとadvicesへの保存は
// 1回だけであることを確認する(docs/implementation-plan.md 9-6 課題A)。
// 排他制御が無い現状では、全リクエストが「提案なし」と判断して全員が生成する。
func TestGetLatestConcurrentRequestsGenerateOnce(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)

	userRepo := repository.NewUserRepository(pool)
	runRepo := repository.NewRunRepository(pool)
	goalRepo := repository.NewGoalRepository(pool)
	adviceRepo := repository.NewAdviceRepository(pool)

	// テスト専用のユーザーを作る。終了時に削除すれば、子テーブルはON DELETE CASCADEで消える。
	var userID string
	if err := pool.QueryRow(ctx, `SELECT gen_random_uuid()::text`).Scan(&userID); err != nil {
		t.Fatalf("user id の採番に失敗: %v", err)
	}
	if _, err := userRepo.Create(ctx, userID, nil); err != nil {
		t.Fatalf("ユーザー作成に失敗: %v", err)
	}
	t.Cleanup(func() { _ = userRepo.Delete(context.Background(), userID) })

	for i := 0; i < minRunsForAdvice; i++ {
		_, err := runRepo.Create(ctx, &model.Run{
			UserID: userID, DistanceKm: 5, DurationSec: 1500, PaceSecPerKm: 300,
			RunDate: model.NewDate(time.Now()),
		})
		if err != nil {
			t.Fatalf("記録作成に失敗: %v", err)
		}
	}
	_, err := goalRepo.CreateAsNewActive(ctx, &model.Goal{
		UserID: userID, GoalType: "5km", TargetTimeSec: 1400,
		TargetDate: model.NewDate(time.Now().AddDate(0, 2, 0)),
	})
	if err != nil {
		t.Fatalf("目標作成に失敗: %v", err)
	}

	gen := &countingGenerator{delay: 500 * time.Millisecond}
	svc := NewAdviceService(adviceRepo, NewUserService(userRepo, "", ""), runRepo, goalRepo, noWeather{}, gen)

	const requests = 5
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make(chan error, requests)
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start // 全員が揃ってから一斉に呼ぶ(複数タブの同時操作の再現)
			if _, err := svc.GetLatest(ctx, userID); err != nil {
				errs <- err
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("GetLatest がエラーを返した: %v", err)
	}

	var saved int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM advices WHERE user_id = $1`, userID).Scan(&saved); err != nil {
		t.Fatalf("advicesの件数取得に失敗: %v", err)
	}
	if got := gen.calls.Load(); got != 1 {
		t.Errorf("AIの呼び出し回数 = %d, 期待値 1", got)
	}
	if saved != 1 {
		t.Errorf("advicesの保存件数 = %d, 期待値 1", saved)
	}
}
