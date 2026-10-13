// APIの日付は"YYYY-MM-DD"の文字列。new Date("2026-09-24")はUTCの0時として解釈され、
// 環境によって日付がずれうるため、文字列のまま年・月・日に分解して扱う。
function parseYmd(ymd: string): { y: number; m: number; d: number } {
  // 将来APIが"2026-09-24T00:00:00Z"のような時刻付きで返しても壊れないよう、先頭の日付部分だけを使う。
  const [y, m, d] = ymd.slice(0, 10).split('-').map(Number)
  return { y, m, d }
}

// 一覧の日付表示。同じ年なら年を省いて "9/24" にして幅を節約し、
// 年が違う場合だけ "2025/9/24" と年を付ける(フェーズ9-3)。
export function formatShortDate(ymd: string, today: Date = new Date()): string {
  const { y, m, d } = parseYmd(ymd)
  if (Number.isNaN(y + m + d)) return ymd
  return y === today.getFullYear() ? `${m}/${d}` : `${y}/${m}/${d}`
}

// 目標日までの残り日数。今日が目標日なら0、過ぎていれば負の数。
// 端末のローカル日付で、時刻を切り捨てた日単位で数える(夏時間の無い日本向けのため
// 日数の差は24時間の倍数として丸めて求める)。
export function daysUntil(ymd: string, today: Date = new Date()): number {
  const { y, m, d } = parseYmd(ymd)
  const target = Date.UTC(y, m - 1, d)
  const now = Date.UTC(today.getFullYear(), today.getMonth(), today.getDate())
  return Math.round((target - now) / 86_400_000)
}
