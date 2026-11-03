package service

import (
	"context"
	"errors"
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
// 割り込める状態を作る。failFirstがtrueなら、最初の1回だけ失敗する。
type countingGenerator struct {
	calls     atomic.Int32
	delay     time.Duration
	failFirst bool
}

func (g *countingGenerator) GenerateAdvice(ctx context.Context, _ AdvicePromptInput) (AdviceGeneration, error) {
	n := g.calls.Add(1)
	select {
	case <-time.After(g.delay):
	case <-ctx.Done():
		return AdviceGeneration{}, ctx.Err()
	}
	if g.failFirst && n == 1 {
		return AdviceGeneration{}, ErrExternalAPI
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

// newAdviceFixture は「記録3件・有効な目標あり・提案なし」のテスト専用ユーザーと、
// そのユーザーに対するAdviceServiceを作る。終了時にユーザーを削除すれば、
// 子テーブルはON DELETE CASCADEで消える。
func newAdviceFixture(t *testing.T, gen AdviceGenerator) (svc *AdviceService, pool *pgxpool.Pool, userID string) {
	t.Helper()
	ctx := context.Background()
	pool = newTestPool(t)

	userRepo := repository.NewUserRepository(pool)
	runRepo := repository.NewRunRepository(pool)
	goalRepo := repository.NewGoalRepository(pool)
	adviceRepo := repository.NewAdviceRepository(pool)

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

	svc = NewAdviceService(adviceRepo, NewUserService(userRepo, "", ""), runRepo, goalRepo, noWeather{}, gen)
	return svc, pool, userID
}

func countAdvices(t *testing.T, pool *pgxpool.Pool, userID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM advices WHERE user_id = $1`, userID).Scan(&n); err != nil {
		t.Fatalf("advicesの件数取得に失敗: %v", err)
	}
	return n
}

// 同じユーザーの GetLatest が同時に何度呼ばれても、AIの呼び出しとadvicesへの保存は
// 1回だけであることを確認する(docs/implementation-plan.md 9-6 課題A、docs/adr/031)。
// 排他制御が無かった頃は、全リクエストが「提案なし」と判断して全員が生成していた
// (修正前の実測: AI呼び出し5回・保存5件)。
func TestGetLatestConcurrentRequestsGenerateOnce(t *testing.T) {
	ctx := context.Background()
	gen := &countingGenerator{delay: 500 * time.Millisecond}
	svc, pool, userID := newAdviceFixture(t, gen)

	const requests = 5
	var wg sync.WaitGroup
	start := make(chan struct{})
	statuses := make(chan AdviceStatus, requests)
	errs := make(chan error, requests)
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start // 全員が揃ってから一斉に呼ぶ(複数タブの同時操作の再現)
			res, err := svc.GetLatest(ctx, userID)
			if err != nil {
				errs <- err
				return
			}
			statuses <- res.Status
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	close(statuses)
	for err := range errs {
		t.Errorf("GetLatest がエラーを返した: %v", err)
	}

	if got := gen.calls.Load(); got != 1 {
		t.Errorf("AIの呼び出し回数 = %d, 期待値 1", got)
	}
	if saved := countAdvices(t, pool, userID); saved != 1 {
		t.Errorf("advicesの保存件数 = %d, 期待値 1", saved)
	}
	// 生成できなかったリクエストは、エラーではなく「生成中」か、完成済みの提案を受け取る。
	for st := range statuses {
		if st != AdviceStatusReady && st != AdviceStatusGenerating {
			t.Errorf("想定外のstatus: %q", st)
		}
	}
}

// 生成に失敗しても生成の権利が戻り、ユーザーが再読み込みですぐにやり直せることを確認する。
// 戻らないと、権利の期限(3分)が切れるまで「生成中」のままになってしまう。
func TestGetLatestReleasesLeaseWhenGenerationFails(t *testing.T) {
	ctx := context.Background()
	gen := &countingGenerator{delay: 10 * time.Millisecond, failFirst: true}
	svc, pool, userID := newAdviceFixture(t, gen)

	if _, err := svc.GetLatest(ctx, userID); !errors.Is(err, ErrExternalAPI) {
		t.Fatalf("1回目: ErrExternalAPI を期待したが %v", err)
	}
	res, err := svc.GetLatest(ctx, userID)
	if err != nil {
		t.Fatalf("2回目がエラー: %v", err)
	}
	if res.Status != AdviceStatusReady || res.Advice == nil {
		t.Errorf("2回目: status=%q advice=%v, ready と提案を期待", res.Status, res.Advice)
	}
	if got := gen.calls.Load(); got != 2 {
		t.Errorf("AIの呼び出し回数 = %d, 期待値 2", got)
	}
	if saved := countAdvices(t, pool, userID); saved != 1 {
		t.Errorf("advicesの保存件数 = %d, 期待値 1", saved)
	}
}

// 権利は、他のリクエストが持っている間は取れず、期限が切れれば取れる。
// また、期限が切れて別のリクエストが新しく取った権利を、古い持ち主が返しても消えない
// (docs/adr/031。レビュー指摘: 無条件に消すと、生成中のリクエストの権利まで消えて二重生成になる)。
func TestGenerationLeaseExpiryAndOwnership(t *testing.T) {
	ctx := context.Background()
	_, pool, userID := newAdviceFixture(t, &countingGenerator{})
	repo := repository.NewAdviceRepository(pool)

	first, err := repo.TryAcquireGenerationLease(ctx, userID, time.Minute)
	if err != nil || first == nil {
		t.Fatalf("1回目は取れるはず: lease=%v err=%v", first, err)
	}
	if second, err := repo.TryAcquireGenerationLease(ctx, userID, time.Minute); err != nil || second != nil {
		t.Fatalf("持ち主がいる間は取れないはず: lease=%v err=%v", second, err)
	}

	// 期限切れを再現するため、期限を過去に書き換える(本来は時間が経って切れる)。
	if _, err := pool.Exec(ctx, `UPDATE users SET advice_lock_until = now() - interval '1 second' WHERE id = $1`, userID); err != nil {
		t.Fatalf("期限の書き換えに失敗: %v", err)
	}
	second, err := repo.TryAcquireGenerationLease(ctx, userID, time.Minute)
	if err != nil || second == nil {
		t.Fatalf("期限切れなら別のリクエストが取れるはず: lease=%v err=%v", second, err)
	}

	// 期限切れの古い持ち主(first)が返しても、新しい持ち主(second)の権利は消えない。
	if err := repo.ReleaseGenerationLease(ctx, first); err != nil {
		t.Fatalf("古い持ち主の解放に失敗: %v", err)
	}
	if third, err := repo.TryAcquireGenerationLease(ctx, userID, time.Minute); err != nil || third != nil {
		t.Fatalf("古い持ち主の解放で、新しい権利が消えている: lease=%v err=%v", third, err)
	}

	// 本人が返せば、次のリクエストが取れる。
	if err := repo.ReleaseGenerationLease(ctx, second); err != nil {
		t.Fatalf("本人の解放に失敗: %v", err)
	}
	if again, err := repo.TryAcquireGenerationLease(ctx, userID, time.Minute); err != nil || again == nil {
		t.Fatalf("本人が返したあとは取れるはず: lease=%v err=%v", again, err)
	}
}
