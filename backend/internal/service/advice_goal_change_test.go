package service

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/masa2050/pacely/backend/internal/model"
	"github.com/masa2050/pacely/backend/internal/repository"
)

// newGoalForTest は、テスト用ユーザーの有効な目標を新しいものに切り替える。
// 本番の目標作成(POST /goals)と同じく、前の目標は自動でabandonedになる(docs/adr/009)。
func newGoalForTest(t *testing.T, pool *pgxpool.Pool, userID string) *model.Goal {
	t.Helper()
	goal, err := repository.NewGoalRepository(pool).CreateAsNewActive(context.Background(), &model.Goal{
		UserID: userID, GoalType: "10km", TargetTimeSec: 3000,
		TargetDate: model.NewDate(time.Now().AddDate(0, 3, 0)),
	})
	if err != nil {
		t.Fatalf("新しい目標の作成に失敗: %v", err)
	}
	return goal
}

// 目標を切り替えたら、前回の生成から24時間以内でも、新しい目標向けに再生成されることを確認する
// (docs/implementation-plan.md 9-6 課題B、docs/adr/034)。
// 修正前は、放棄済みの目標向けの提案がそのまま返っていた。
func TestGetLatestRegeneratesWhenGoalChanges(t *testing.T) {
	ctx := context.Background()
	gen := &countingGenerator{delay: 10 * time.Millisecond}
	svc, pool, userID := newAdviceFixture(t, gen)

	first, err := svc.GetLatest(ctx, userID)
	if err != nil || first.Advice == nil {
		t.Fatalf("1回目: advice=%v err=%v", first, err)
	}

	// 目標が同じなら、24時間以内はキャッシュを返す(従来どおり)。
	cached, err := svc.GetLatest(ctx, userID)
	if err != nil || cached.Advice == nil || cached.Advice.ID != first.Advice.ID {
		t.Fatalf("同じ目標ではキャッシュを期待: advice=%v err=%v", cached, err)
	}
	if got := gen.calls.Load(); got != 1 {
		t.Fatalf("同じ目標でのAI呼び出し回数 = %d, 期待値 1", got)
	}

	newGoal := newGoalForTest(t, pool, userID)
	res, err := svc.GetLatest(ctx, userID)
	if err != nil || res.Status != AdviceStatusReady || res.Advice == nil {
		t.Fatalf("目標変更後: res=%v err=%v", res, err)
	}
	if res.Advice.ID == first.Advice.ID {
		t.Error("目標を変えたのに、古い目標向けの提案が返った")
	}
	if res.Advice.GoalID == nil || *res.Advice.GoalID != newGoal.ID {
		t.Errorf("提案のgoal_id = %v, 期待値 %s", res.Advice.GoalID, newGoal.ID)
	}
	if got := gen.calls.Load(); got != 2 {
		t.Errorf("AI呼び出し回数 = %d, 期待値 2", got)
	}
	if saved := countAdvices(t, pool, userID); saved != 2 {
		t.Errorf("advicesの保存件数 = %d, 期待値 2", saved)
	}
}

// 目標の切り替えを繰り返しても、24時間に生成できるのはmaxAdviceGenerationsPerWindow回までで、
// 上限に達したら、AIを呼ばずに直前の提案を返すことを確認する(docs/adr/034)。
// 上限が無いと、目標を作り直すだけでAI APIの枠を使い切れてしまう。
func TestGetLatestCapsRegenerationsPerWindow(t *testing.T) {
	ctx := context.Background()
	gen := &countingGenerator{delay: 10 * time.Millisecond}
	svc, pool, userID := newAdviceFixture(t, gen)

	var last *model.Advice
	for i := 0; i < maxAdviceGenerationsPerWindow; i++ {
		if i > 0 {
			newGoalForTest(t, pool, userID)
		}
		res, err := svc.GetLatest(ctx, userID)
		if err != nil || res.Advice == nil {
			t.Fatalf("%d回目: res=%v err=%v", i+1, res, err)
		}
		last = res.Advice
	}
	if got := gen.calls.Load(); got != maxAdviceGenerationsPerWindow {
		t.Fatalf("上限までのAI呼び出し回数 = %d, 期待値 %d", got, maxAdviceGenerationsPerWindow)
	}

	// 上限に達した後に目標を変えても、生成せずに直前の提案を返す。
	newGoalForTest(t, pool, userID)
	res, err := svc.GetLatest(ctx, userID)
	if err != nil || res.Status != AdviceStatusReady || res.Advice == nil {
		t.Fatalf("上限到達後: res=%v err=%v", res, err)
	}
	if res.Advice.ID != last.ID {
		t.Errorf("直前の提案(%s)を期待したが %s が返った", last.ID, res.Advice.ID)
	}
	if got := gen.calls.Load(); got != maxAdviceGenerationsPerWindow {
		t.Errorf("上限到達後にAIが呼ばれた: 呼び出し回数 = %d", got)
	}
	if saved := countAdvices(t, pool, userID); saved != maxAdviceGenerationsPerWindow {
		t.Errorf("advicesの保存件数 = %d, 期待値 %d", saved, maxAdviceGenerationsPerWindow)
	}
}

// goal_idがNULLの提案は、どの目標向けか確認できないので、24時間以内でも再生成する(docs/adr/034)。
func TestGetLatestRegeneratesWhenAdviceGoalIsNull(t *testing.T) {
	ctx := context.Background()
	gen := &countingGenerator{delay: 10 * time.Millisecond}
	svc, pool, userID := newAdviceFixture(t, gen)

	if _, err := svc.GetLatest(ctx, userID); err != nil {
		t.Fatalf("1回目がエラー: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE advices SET goal_id = NULL WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("goal_idのNULL化に失敗: %v", err)
	}

	res, err := svc.GetLatest(ctx, userID)
	if err != nil || res.Advice == nil || res.Advice.GoalID == nil {
		t.Fatalf("再生成された提案を期待: res=%v err=%v", res, err)
	}
	if got := gen.calls.Load(); got != 2 {
		t.Errorf("AI呼び出し回数 = %d, 期待値 2", got)
	}
}

// 権利を取った後の再確認(double-checked locking)でも、同じ条件が効くことを確認する。
// 権利を取る前と後の確認は同じlatestIfFreshを通るので、これを直接呼んで検証する。
// 片方だけ条件が抜けていると、並行したリクエストが古い目標向けの提案を「新しい」と返してしまう。
func TestLatestIfFreshChecksGoal(t *testing.T) {
	ctx := context.Background()
	gen := &countingGenerator{delay: 10 * time.Millisecond}
	svc, pool, userID := newAdviceFixture(t, gen)

	first, err := svc.GetLatest(ctx, userID)
	if err != nil || first.Advice == nil || first.Advice.GoalID == nil {
		t.Fatalf("1回目: res=%v err=%v", first, err)
	}
	oldGoalID := *first.Advice.GoalID

	if got, err := svc.latestIfFresh(ctx, userID, oldGoalID); err != nil || got == nil {
		t.Errorf("同じ目標では提案を返すはず: advice=%v err=%v", got, err)
	}
	newGoal := newGoalForTest(t, pool, userID)
	if got, err := svc.latestIfFresh(ctx, userID, newGoal.ID); err != nil || got != nil {
		t.Errorf("別の目標では(nil, nil)のはず: advice=%v err=%v", got, err)
	}
}
