// 時間帯に応じた挨拶メッセージ(フェーズ6・フロントエンドのみで完結する任意機能)。
// バックエンドとは無関係のUI装飾のため、docs/api.mdのエンドポイント一覧には現れない。
export function getGreeting(date: Date = new Date()): string {
  const hour = date.getHours()

  if (hour >= 5 && hour < 10) {
    return 'おはようございます'
  }
  if (hour >= 10 && hour < 17) {
    return 'こんにちは'
  }
  if (hour >= 17 && hour < 22) {
    return 'こんばんは'
  }
  return '夜遅くまでお疲れさまです'
}
