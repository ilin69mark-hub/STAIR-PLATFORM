'use client'

// Калькулятор на витрине — тонкий клиентский остров поверх конструктора
// frontend-store (`@store/components/Constructor`). Логика не копируется:
// форма, живая валидация, «Спасти расчёт», 3D-вьювер и работа с API — тот
// же код, что и на витрине-калькуляторе, поэтому расхождений между сайтами
// не будет. Браузер обращается к /api, который Next проксирует на Go-API.
import { useEffect, useState } from 'react'
import { Constructor } from '@store/components/Constructor'
// Конструктор читает пользователя из контекста витрины-калькулятора, поэтому
// остов (AuthProvider) тоже переиспользуется: сессия одна и та же (cookie
// домена), логика входа не дублируется.
import { AuthProvider } from '@store/auth/AuthContext'
import { defaultConfig } from '@shared/config'
import '@store/App.css'

export function Calculator({ initialMaterial }: { initialMaterial?: string }) {
  // Материал можно задать ссылкой с карточки материала:
  // /calculator?material=WOOD-WALNUT
  const [ready, setReady] = useState(false)
  useEffect(() => setReady(true), [])
  if (!ready) return <div className="scene__loading">Загрузка калькулятора…</div>
  return (
    <div data-testid="showcase-calculator" data-default-material={initialMaterial ?? defaultConfig.material}>
      <AuthProvider>
        <Constructor />
      </AuthProvider>
    </div>
  )
}
