import { useState } from 'react'
import { COOKIE_CONSENT_KEY } from '../config'

interface Props {
  onOpenPolicy: () => void
}

// Баннер согласия на использование cookie (EU-стиль). Решение хранится в
// localStorage; после согласия баннер больше не показывается.
export function CookieBanner({ onOpenPolicy }: Props) {
  const [hidden, setHidden] = useState<boolean>(() => {
    try {
      return localStorage.getItem(COOKIE_CONSENT_KEY) === 'accepted'
    } catch {
      return false
    }
  })

  if (hidden) {
    return null
  }

  const accept = () => {
    try {
      localStorage.setItem(COOKIE_CONSENT_KEY, 'accepted')
    } catch {
      // localStorage недоступен (приватный режим) — просто скрываем баннер.
    }
    setHidden(true)
  }

  return (
    <div className="cookie-banner" role="region" aria-label="Согласие на cookie">
      <p>
        Мы используем cookie для корректной работы сайта и анализа посещаемости.{' '}
        <a href="#cookies" onClick={(e) => { e.preventDefault(); onOpenPolicy() }}>
          Подробнее о политике cookie
        </a>
      </p>
      <div className="cookie-banner-actions">
        <button className="sp-btn sp-btn--primary" onClick={accept}>
          Принять
        </button>
      </div>
    </div>
  )
}