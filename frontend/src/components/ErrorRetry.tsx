type Props = {
  message: string
  onRetry: () => void
}

// フェーズ9-4: データ取得に失敗したときの表示。以前は多くのカードがエラー時に
// エラー文だけを返していたため、ボタンごと消えてページを再読み込みしないと
// 復帰できなかった。エラー文と一緒に必ず再取得の手段を出す。
// 9-5でAdviceHistoryに個別に入れていた表示を、各カードで揃えるため共通化した。
export function ErrorRetry({ message, onRetry }: Props) {
  return (
    <div className="error-retry">
      <p className="dashboard__error">{message}</p>
      <button type="button" onClick={onRetry}>
        再読み込み
      </button>
    </div>
  )
}
