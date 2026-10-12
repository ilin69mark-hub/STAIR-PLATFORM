import { render } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { analyticsApi } from '../../api/analytics'
import { AuthProvider } from '../../auth/AuthContext'
import { SECTION_IDS, SECTION_RENDERERS, renderSection } from './sections'

// Разделы админки — тонкие обёртки над useReport: каждая тянет свой срез
// аналитики и передаёт результат панели. Тест рендерит КАЖДЫЙ раздел из
// реестра, чтобы добавление нового не осталось незамеченным, и чтобы
// опечатка в id не превратилась в тихо пустую страницу.
vi.mock('../../api/analytics', () => ({
  analyticsApi: new Proxy(
    {},
    {
      get: () => vi.fn().mockResolvedValue([]),
    },
  ),
}))

afterEach(() => {
  vi.clearAllMocks()
})

describe('реестр разделов', () => {
  it('id разделов совпадают с ключами рендереров', () => {
    // SECTION_IDS питает плитки. Расхождение с реестром означало бы плитку,
    // открывающуюся в пустоту.
    expect([...SECTION_IDS].sort()).toEqual(Object.keys(SECTION_RENDERERS).sort())
  })

  it('id уникальны', () => {
    // Дубль id тихо перетирал бы одну секцию другой.
    expect(new Set(SECTION_IDS).size).toBe(SECTION_IDS.length)
  })

  it('каждый раздел рендерится без исключения', async () => {
    for (const id of SECTION_IDS) {
      const { unmount } = render(
        <AuthProvider>{renderSection(id, { user: null })}</AuthProvider>,
      )
      unmount()
    }
    expect(SECTION_IDS.length).toBeGreaterThan(15)
  })

  it('рендерит каждый раздел и с заданным пользователем', () => {
    // user не null — ветка с правами: панель решает, показывать ли кнопки.
    const user = { id: 'u-1', tenant_id: 't-1', role: 'admin' } as never
    for (const id of SECTION_IDS) {
      const { unmount } = render(
        <AuthProvider>{renderSection(id, { user })}</AuthProvider>,
      )
      unmount()
    }
    expect(analyticsApi).toBeDefined()
  })

  it('неизвестный раздел даёт null, а не падает', () => {
    // Опечатка в id из сохранённых настроек не должна ронять всю страницу.
    expect(renderSection('не-такого-раздела', { user: null })).toBeNull()
  })
})