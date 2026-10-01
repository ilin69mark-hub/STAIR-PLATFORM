'use client'

// Калькулятор на витрине — тонкий клиентский остров поверх конструктора
// frontend-store (`@store/components/Constructor`). Логика не копируется:
// форма, живая валидация, «Спасти расчёт», 3D-вьювер и работа с API — тот
// же код, что и на витрине-калькуляторе, поэтому расхождений между сайтами
// не будет. Браузер обращается к /api, который Next проксирует на Go-API.
import { useEffect, useState } from 'react'
import { Constructor } from '@shared/storefront/components/Constructor'
// Конструктор читает пользователя из контекста витрины-калькулятора, поэтому
// остов (AuthProvider) тоже переиспользуется: сессия одна и та же (cookie
// домена), логика входа не дублируется.
import { StorefrontProviders } from '@shared/storefront/StorefrontProviders'
import { defaultConfig } from '@shared/config'
// Стили: только база, конструктор и 3D-вьювер. Лендинг, кабинет, формы
// авторизации витрине не нужны — раньше она тянула весь App.css целиком.
import '@shared/storefront/styles/base.css'
import '@shared/storefront/styles/constructor.css'
import '@shared/storefront/styles/viewer.css'

export function Calculator({ initialMaterial }: { initialMaterial?: string }) {
  // Материал можно задать ссылкой с карточки материала:
  // /calculator?material=WOOD-WALNUT
  const [ready, setReady] = useState(false)
  useEffect(() => setReady(true), [])
  if (!ready) return <div className="scene__loading">Загрузка калькулятора…</div>
  return (
    <div data-testid="showcase-calculator" data-default-material={initialMaterial ?? defaultConfig.material}>
      <StorefrontProviders>
        <Constructor />
      </StorefrontProviders>
    </div>
  )
}
