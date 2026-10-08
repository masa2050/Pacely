import { useEffect, useState } from 'react'
import type { Session } from '@supabase/supabase-js'
import {
  getMe,
  listRuns,
  listGoals,
  getActiveGoal,
  sortRuns,
  type Me,
  type Run,
  type Goal,
} from '../lib/api'
import { supabase } from '../lib/supabase'
import { getGreeting } from '../lib/greeting'
import { RunForm } from './RunForm'
import { RunList } from './RunList'
import { GoalForm } from './GoalForm'
import { GoalList } from './GoalList'
import { ProgressView } from './ProgressView'
import { AdviceView } from './AdviceView'
import { Sidebar } from './Sidebar'
import { toUserMessage } from '../lib/errors'
import { ErrorRetry } from './ErrorRetry'

type Props = {
  session: Session
  onSettingsClick: () => void
  onHistoryClick: () => void
}

// AI提案を生成できる最小の記録件数。BE(service/advice.goのminRunsForAdvice)と同じ値にする。
// AdviceViewが「新規生成中」の表示を出してよいか(=生成が起こりうるか)の判定にだけ使う。
const MIN_RUNS_FOR_ADVICE = 3

// フェーズ1の完了条件確認用: ログイン後、JWTを使って GET/PUT /users/me が
// 実際に動くことをUI上で見えるようにする最小限の画面。
export function Dashboard({ session, onSettingsClick, onHistoryClick }: Props) {
  const [me, setMe] = useState<Me | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [runs, setRuns] = useState<Run[]>([])
  const [runsError, setRunsError] = useState<string | null>(null)
  const [goals, setGoals] = useState<Goal[]>([])
  const [activeGoal, setActiveGoal] = useState<Goal | null>(null)
  const [goalsError, setGoalsError] = useState<string | null>(null)
  // 失敗後の再読み込み中だけ「読み込み中...」を出すためのstate。初回読み込みでは使わない
  // (従来どおり空の状態から表示し、ちらつきを増やさない)。
  const [runsReloading, setRunsReloading] = useState(false)
  const [goalsReloading, setGoalsReloading] = useState(false)

  // フェーズ9-4: 取得処理をカード単位の関数に切り出し、初回(useEffect)と
  // 失敗時の再読み込み(ErrorRetry)の両方から呼べるようにした。以前はuseEffect内に
  // 直接書いていたため、失敗するとページ全体を再読み込みするしか復帰手段がなかった。
  // どの関数も内部でcatchしてエラー文をstateに入れるため、返すPromiseは常にresolveする
  // (呼び出し側はfinallyで再読み込み中表示を戻すだけでよい)。
  function loadMe() {
    return getMe()
      .then(setMe)
      .catch((err) => setError(toUserMessage(err)))
  }

  function loadRuns() {
    return listRuns()
      .then(setRuns)
      .catch((err) => setRunsError(toUserMessage(err)))
  }

  function loadGoalList() {
    return listGoals()
      .then(setGoals)
      .catch((err) => setGoalsError(toUserMessage(err)))
  }

  function loadGoals() {
    return Promise.all([
      loadGoalList(),
      getActiveGoal()
        .then(setActiveGoal)
        .catch((err) => setGoalsError(toUserMessage(err))),
    ])
  }

  // 再読み込みボタン用。エラーを消してから取り直す処理をここに集約している。
  function retryMe() {
    setError(null)
    loadMe()
  }

  function retryRuns() {
    setRunsError(null)
    setRunsReloading(true)
    loadRuns().finally(() => setRunsReloading(false))
  }

  function retryGoals() {
    setGoalsError(null)
    setGoalsReloading(true)
    loadGoals().finally(() => setGoalsReloading(false))
  }

  useEffect(() => {
    loadMe()
    loadRuns()
    loadGoals()
  }, [])

  return (
    <div className="dashboard">
      <header className="dashboard__header">
        <div className="dashboard__header-left">
          <Sidebar onSettingsClick={onSettingsClick} onHistoryClick={onHistoryClick} />
          <h1>Pacely</h1>
        </div>
        <button type="button" onClick={() => supabase.auth.signOut()}>
          ログアウト
        </button>
      </header>

      <p className="dashboard__greeting">
        {getGreeting()}
        {/* ユーザーネーム未設定の間は、フェーズ7-3以前と同じくメールアドレスの
            ローカル部を仮の表示名として使う(docs/implementation-plan.md 7-3)。 */}
        {me?.username
          ? `、${me.username}さん`
          : session.user.email
            ? `、${session.user.email.split('@')[0]}さん`
            : ''}
      </p>

      {error && <ErrorRetry message={error} onRetry={retryMe} />}

      {/* PC幅(1024px〜)では3カラム表示にする(左:プロフィール・記録/中央:目標・進捗/右:AI提案)。
          CSS Gridの行スパンで実装すると、カード同士の内容量の差(AI提案は長文になりがち)が
          そのまま他列の空白になってしまったため、列ごとに高さが独立するFlexboxに変更した
          (docs/adr/015)。SP幅ではdashboard__columnをdisplay:contentsにして、
          これまで通りカードが縦一列に並ぶ見た目に戻す。 */}
      <div className="dashboard__column dashboard__column--left">
        {me ? (
          <div className="dashboard__card">
            <h2>プロフィール</h2>
            {/* user id・created_atはユーザーにとって意味のない情報のため非表示にした
                (docs/implementation-plan.md 9-1)。ユーザーネームは挨拶文にも出ているが、
                このカードでは編集への導線(設定画面)として値そのものを表示しておく。
                dtラベルはDBカラム名(username/region)の生表記をやめ、日本語に統一した。 */}
            <dl>
              <dt>ユーザーネーム</dt>
              <dd>{me.username ?? '(未設定)'}</dd>
              <dt>地域</dt>
              <dd>{me.region ?? '(未設定)'}</dd>
            </dl>
            {/* 地域の編集・退会は設定画面に一本化した(フェーズ7-2, Settings.tsx)。
                ここは表示専用のカードとして残す。 */}
          </div>
        ) : (
          !error && <p>読み込み中...</p>
        )}

        <div id="dashboard-runs" className="dashboard__card">
          <h2>ランニング記録</h2>
          {runsReloading && <p>読み込み中...</p>}
          {runsError && <ErrorRetry message={runsError} onRetry={retryRuns} />}
          <RunForm onSaved={(run) => setRuns((prev) => sortRuns([run, ...prev]))} />
          <RunList runs={runs} onChanged={setRuns} />
        </div>
      </div>

      <div className="dashboard__column dashboard__column--center">
        <div id="dashboard-goals" className="dashboard__card">
          <h2>目標設定</h2>
          {goalsReloading && <p>読み込み中...</p>}
          {goalsError && <ErrorRetry message={goalsError} onRetry={retryGoals} />}
          <GoalList
            activeGoal={activeGoal}
            goals={goals}
            onActiveChanged={setActiveGoal}
            onGoalsChanged={setGoals}
          />
          <GoalForm
            onSaved={(goal) => {
              // 新規作成時、既存のactive目標はBE側で自動的にabandonedへ更新される
              // (docs/adr/009)。ローカル側の反映漏れを防ぐため一覧ごと取り直す。
              setActiveGoal(goal)
              loadGoalList()
            }}
          />
        </div>

        <div className="dashboard__card">
          <h2>進捗確認</h2>
          {/* activeGoalの切り替わりやrunsの増減のたびに進捗を取り直したいので、
              それらをkeyにしてコンポーネントごと再マウントする(簡易的な再取得トリガー)。 */}
          <ProgressView key={`${activeGoal?.id ?? 'none'}-${runs.length}`} />
        </div>
      </div>

      <div className="dashboard__column dashboard__column--right">
        <div id="dashboard-advice" className="dashboard__card">
          <h2>AI提案</h2>
          {/* activeGoal・runsの変化に応じて再取得したいので、Progressと同様keyで再マウントする。
              ただし取得自体は「前回生成から24時間以内ならキャッシュを返す」ため、
              毎回AIが呼ばれるわけではない(docs/adr/004)。 */}
          <AdviceView
            key={`${activeGoal?.id ?? 'none'}-${runs.length}`}
            onHistoryClick={onHistoryClick}
            canGenerate={runs.length >= MIN_RUNS_FOR_ADVICE && activeGoal !== null}
          />
        </div>
      </div>
    </div>
  )
}
