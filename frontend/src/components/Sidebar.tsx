import { useState } from 'react'

type Props = {
  onSettingsClick: () => void
}

// フェーズ7-2: サイドバーナビゲーション。
// 「記録/目標/AI提案」はDashboard.tsx内の既存カードへのアンカースクロールとし、
// 別画面に分割しない(Dashboard.tsxのカード構成・並び順は変えない方針)。
// 「設定」だけは退会機能の移動先として新設した別画面のため、コールバックで画面遷移する。
// SP幅ではハンバーガーメニューとして開閉し、PC幅(1024px〜)では常時表示の固定サイドバーになる(App.css参照)。
export function Sidebar({ onSettingsClick }: Props) {
  const [open, setOpen] = useState(false)

  function closeMenu() {
    setOpen(false)
  }

  return (
    <>
      <button
        type="button"
        className="sidebar__toggle"
        onClick={() => setOpen((v) => !v)}
        aria-label="メニュー"
        aria-expanded={open}
      >
        ☰ メニュー
      </button>

      <nav className={`sidebar ${open ? 'sidebar--open' : ''}`}>
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
            onSettingsClick()
          }}
        >
          設定
        </button>
      </nav>
    </>
  )
}
