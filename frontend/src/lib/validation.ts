import type { FormEvent } from 'react'

// フェーズ9-3: ブラウザ標準のバリデーションメッセージ("Please fill out this field."等)は
// ブラウザの表示言語に従うため、日本語UIの中に英語が出ることがある。ブラウザのメッセージを
// 日本語に差し替える(setCustomValidity)。検証のルール自体(required/min/max等)は
// 各<input>の属性のまま変えない。
//
// `invalid`イベントはバブリングしないが、キャプチャ(Capture)なら祖先の<form>で
// まとめて受けられるため、フォームに下記のハンドラを付けるだけで全入力欄に効く。
function messageFor(el: HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement): string {
  const v = el.validity
  if (v.valueMissing) return el instanceof HTMLSelectElement ? '選択してください' : '入力してください'
  if (v.typeMismatch) {
    return el instanceof HTMLInputElement && el.type === 'email'
      ? 'メールアドレスの形式で入力してください'
      : '正しい形式で入力してください'
  }
  // minLength/maxLengthは<select>に無いため、<input>と<textarea>に絞って参照する。
  if (!(el instanceof HTMLSelectElement)) {
    if (v.tooShort) return `${el.minLength}文字以上で入力してください`
    if (v.tooLong) return `${el.maxLength}文字以内で入力してください`
  }
  if (el instanceof HTMLInputElement) {
    if (v.rangeUnderflow) return `${el.min}以上の値を入力してください`
    if (v.rangeOverflow) return `${el.max}以下の値を入力してください`
    if (v.stepMismatch) return '入力できる値の刻みに合っていません'
    if (v.badInput) return '正しい値を入力してください'
  }
  if (v.patternMismatch) return '正しい形式で入力してください'
  return ''
}

type FieldEvent = FormEvent<HTMLFormElement>

// <form {...japaneseValidation}> の形で使う。
export const japaneseValidation = {
  onInvalidCapture(e: FieldEvent) {
    const el = e.target as HTMLInputElement
    // 先に空に戻してから判定する。残っていると、直前に付けた独自メッセージ(customError)
    // が「入力済みでも無効」として残り続けるため。
    el.setCustomValidity('')
    el.setCustomValidity(messageFor(el))
  },
  // 入力し直したら独自メッセージを消す。消さないと、値が直っても無効扱いのままになる。
  onInputCapture(e: FieldEvent) {
    ;(e.target as HTMLInputElement).setCustomValidity('')
  },
}
