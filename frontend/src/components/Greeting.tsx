import { useEffect, useState } from 'react'
import { getGreeting } from '../lib/greeting'

type Props = {
  // 「、〇〇さん」の部分。未設定なら空文字。
  suffix: string
}

// フェーズ9-3: 日付を小さく上に置き、「あいさつ、名前さん」と時間帯に合わせた一言の2行を続ける。
// 日付を名前の後ろに並べると文の一部のように見えて位置が不自然だったため、ダッシュボードで
// よく使われる「見出しの上の小さなラベル」の形にした。
//
// タブを開いたまま時間帯や日付をまたいでも古い挨拶・日付が残らないよう、1分ごとに計算し直す。
// 一言は時間帯と日付が変わるときにしか変わらないため、更新してもちらつかない。
// 専用の部品に切り出しているのは、1分ごとの再描画をダッシュボード全体に及ぼさないため。
export function Greeting({ suffix }: Props) {
  const [greeting, setGreeting] = useState(() => getGreeting())

  useEffect(() => {
    const id = setInterval(() => setGreeting(getGreeting()), 60_000)
    return () => clearInterval(id)
  }, [])

  return (
    <div className="dashboard__greeting">
      <p className="dashboard__greeting-date">{greeting.dateLabel}</p>
      <p className="dashboard__greeting-line">
        {greeting.salutation}
        {suffix}
      </p>
      <p className="dashboard__greeting-message">{greeting.message}</p>
    </div>
  )
}
