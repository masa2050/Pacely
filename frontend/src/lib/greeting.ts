// 時間帯に応じた挨拶メッセージ(フェーズ6・フロントエンドのみで完結する任意機能)。
// バックエンドとは無関係のUI装飾のため、docs/api.mdのエンドポイント一覧には現れない。
//
// フェーズ9-3で「あいさつ」+「時間帯に合わせた一言」の2行構成にした。一言は時間帯ごとに
// 複数用意し、日付で選ぶ(同じ日は同じ文になり、画面を開き直しても文が変わってちらつかない)。

type Slot = { salutation: string; messages: string[] }

const MORNING: Slot = {
  salutation: 'おはようございます',
  messages: [
    '朝のランは一日を気持ちよくしてくれます。',
    '今日も自分のペースで走りましょう。',
    '走る前は軽いストレッチから始めましょう。',
  ],
}
const DAYTIME: Slot = {
  salutation: 'こんにちは',
  messages: [
    '今日の練習メニューはAI提案から確認できます。',
    '無理のない範囲で、今日も一歩ずつ。',
    '走ったら、記録を忘れずにつけておきましょう。',
  ],
}
const EVENING: Slot = {
  salutation: 'こんばんは',
  messages: [
    '今日も一日お疲れさまでした。',
    '走った日は、記録を忘れずにつけておきましょう。',
    '夜のランは、明るい服装で安全に走りましょう。',
  ],
}
const NIGHT: Slot = {
  salutation: '夜遅くまでお疲れさまです',
  messages: ['しっかり休むのも大事な練習のうちです。', '睡眠は回復の時間。今日はゆっくり休みましょう。'],
}

function slotFor(hour: number): Slot {
  if (hour >= 5 && hour < 10) return MORNING
  if (hour >= 10 && hour < 17) return DAYTIME
  if (hour >= 17 && hour < 22) return EVENING
  return NIGHT
}

const WEEKDAYS = ['日', '月', '火', '水', '木', '金', '土']

export type Greeting = {
  salutation: string
  message: string
  // "10月14日(火)"
  dateLabel: string
}

export function getGreeting(date: Date = new Date()): Greeting {
  const slot = slotFor(date.getHours())
  // 年始からの通算日(1月1日=1)で一言を選ぶ。Date.UTC(年, 0, 0)は「前年の12月31日」を指すので、
  // 1月1日との差が1になる。日が変われば別の文になる。messagesの件数を増減すると、
  // どの日にどの文が出るかが入れ替わる(件数で割った余りで選んでいるため)。
  const dayOfYear = Math.floor((Date.UTC(date.getFullYear(), date.getMonth(), date.getDate()) - Date.UTC(date.getFullYear(), 0, 0)) / 86_400_000)
  return {
    salutation: slot.salutation,
    message: slot.messages[dayOfYear % slot.messages.length],
    dateLabel: `${date.getMonth() + 1}月${date.getDate()}日(${WEEKDAYS[date.getDay()]})`,
  }
}
