import { useEffect, useState } from 'react'
import { grantConsent, hasConsent } from '@shared/consent'

interface Props {
  onOpenPolicy: () => void
}

// Баннер согласия на обработку данных. ВАЖНО: теперь он не декоративный.
// Раньше кнопка «Принять» писала в localStorage строку 'accepted', которую
// больше не читал никто: ни один счётчик не грузился, cookie не ставились,
// аналитики не было. Баннер врал («анализ посещаемости») и при этом висел
// поверх кнопки «Рассчитать».
//
// Теперь решение управляет сбором: без него не уходят ни события воронки, ни
// Sentry. И решение хранится с ВЕРСИЕЙ политики — при появлении нового
// получателя данных баннер спросит заново, а не навсегда останется скрытым
// у тех, кто нажимал кнопку раньше.
//
// Баннер показывается ТОЛЬКО когда согласия нет. Отзыв согласия — на странице
// политики cookie: держать постоянную плашку «отозвать» значило бы опять
// закрыть собой кнопку «Рассчитать».
export function CookieBanner({ onOpenPolicy }: Props) {
  const [accepted, setAccepted] = useState<boolean>(() => hasConsent())

  useEffect(() => {
    // Согласие могли дать в другой вкладке: localStorage синхронно между
    // вкладками не рассылает события, но «storage» приходит в эту.
    const onStorage = (e: StorageEvent) => {
      if (e.key === 'stair-platform-cookie-consent') setAccepted(hasConsent())
    }
    globalThis.addEventListener?.('storage', onStorage)
    return () => globalThis.removeEventListener?.('storage', onStorage)
  }, [])

  const accept = () => {
    grantConsent()
    setAccepted(true)
  }

  if (accepted) return null

  return (
    <div className="cookie-banner" role="region" aria-label="Согласие на обработку данных">
      <p>
        Чтобы понимать, где посетитель спотыкается и где уходит, мы собираем
        статистику посещений: какие экраны открывают, какие поля оставляют
        пустыми, где расчёт не проходит. Содержимое полей не собираем. Часть
        данных об источнике перехода, стране и устройстве обрабатывает
        Яндекс.Метрика. Пока вы не согласны — не собираем ничего.{' '}
        <a href="#cookies" onClick={(e) => { e.preventDefault(); onOpenPolicy() }}>
          Подробнее о политике
        </a>
      </p>
      <div className="cookie-banner-actions">
        <button className="sp-btn sp-btn--primary" onClick={accept}>
          Разрешить
        </button>
      </div>
    </div>
  )
}
