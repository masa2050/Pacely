import { useState } from 'react'

type Props = {
  onSettingsClick: () => void
  onHistoryClick: () => void
}

// フェーズ7-2: ナビゲーションメニュー。
// 当初PC幅では常時表示の固定サイドバーを検討したが、実装してみると
// ダッシュボードの横幅を圧迫し3カラムレイアウトが狭くなる問題があったため、
// SP/PC共通でハンバーガーボタン+ドロップダウンのシンプルな形に変更した(docs/adr/015)。
// 「記録/目標/AI提案」はDashboard.tsx内の既存カードへのアンカースクロールとし、
// 別画面に分割しない(Dashboard.tsxのカード構成・並び順は変えない方針)。
// 「設定」「提案履歴」は別画面のため、コールバックで画面遷移する
// (提案履歴はフェーズ9-5で追加、docs/adr/020)。
export function Sidebar({ onSettingsClick, onHistoryClick }: Props) {
  const [open, setOpen] = useState(false)

  function closeMenu() {
    setOpen(false)
  }

  return (
    <div className="sidebar">
      <button
        type="button"
        className="sidebar__toggle"
        onClick={() => setOpen((v) => !v)}
        aria-label="メニュー"
        aria-expanded={open}
      >
        ☰
      </button>

      {open && (
        <nav className="sidebar__menu">
          <a href="#dashboard-runs" className="sidebar__link" onClick={closeMenu}>
            記録
          </a>
          <a href="#dashboard-goals" className="sidebar__link" onClick={closeMenu}>
            目標
          </a>
          <a href="#dashboard-advice" className="sidebar__link" onClick={closeMenu}>
            AI提案
          </a>
          <button
            type="button"
            className="sidebar__link sidebar__link--button"
            onClick={() => {
              closeMenu()
              onHistoryClick()
            }}
          >
            提案履歴
          </button>
          <button
            type="button"
            className="sidebar__link sidebar__link--button"
            onClick={() => {
              closeMenu()
              onSettingsClick()
            }}
          >
            設定
          </button>
        </nav>
      )}
    </div>
  )
}
