import { describe, expect, it } from 'vitest'

import { hasCapability, P, hasPermission } from './cabinetPermissions'

// Проверки видимости блоков и разделов уехали в cabinetTiles.test.ts: там живёт
// реестр плиток. Здесь осталась сама модель прав — «есть ли ключ» и уровни.

describe('hasPermission', () => {
  it('раздел без права (needs: null) открыт всем авторизованным', () => {
    expect(hasPermission({ permissions: [] }, null)).toBe(true)
  })

  it('пустые права = прав нет (fail-closed)', () => {
    expect(hasPermission({ permissions: [] }, P.staffView)).toBe(false)
  })

  it('отсутствующее поле permissions = прав нет', () => {
    expect(hasPermission({}, P.staffView)).toBe(false)
    expect(hasPermission(null, P.staffView)).toBe(false)
  })

  it('право из списка даёт доступ', () => {
    expect(hasPermission({ permissions: ['price.materials'] }, P.priceMaterials)).toBe(true)
    expect(hasPermission({ permissions: ['price.materials'] }, P.siteContacts)).toBe(false)
  })

  it('когда есть grants, доверяем им, а не старому списку permissions', () => {
    // Сессия с grants — источник прав. Список permissions в ней может остаться
    // от прежней выдачи, поэтому пересечение двух источников открыло бы
    // разделы, которые сервер уже отозвал.
    const u = {
      grants: [{ capability: P.staffView, level: 'read' as const }],
      permissions: ['security.policy'],
    }
    expect(hasPermission(u, P.staffView)).toBe(true)
    expect(hasPermission(u, P.securityPolicy)).toBe(false)
  })
})

// ---- уровни: read/write/action (EDR-0041) ----
//
// Эти проверки повторяют серверную модель (auth.Satisfies) намеренно: если
// клиент разойдётся с сервером в трактовке «write подразумевает read», то
// либо появится кнопка, которая уйдёт в 403, либо исчезнет рабочая кнопка.

describe('уровни метрики', () => {
  it('write даёт и чтение, action не даёт ничего кроме себя', () => {
    const w = { grants: [{ capability: P.financeSales, level: 'write' as const }] }
    expect(hasCapability(w, P.financeSales, 'read')).toBe(true)
    expect(hasCapability(w, P.financeSales, 'write')).toBe(true)
    expect(hasCapability(w, P.financeSales, 'action')).toBe(false)

    const a = { grants: [{ capability: P.financeRefunds, level: 'action' as const }] }
    expect(hasCapability(a, P.financeRefunds, 'action')).toBe(true)
    expect(hasCapability(a, P.financeRefunds, 'read')).toBe(false)
    expect(hasCapability(a, P.financeRefunds, 'write')).toBe(false)
  })

  it('одна выданная метрика не открывает соседние', () => {
    const u = { grants: [{ capability: P.siteSEO, level: 'write' as const }] }
    expect(hasCapability(u, P.siteSEO, 'write')).toBe(true)
    // Реквизиты — соседняя метрика, её не выдавали.
    expect(hasCapability(u, P.siteCompany, 'write')).toBe(false)
    // Старая сессия без grants: список имён не различает уровень, поэтому
    // действие по такой сессии не считаем разрешённым.
    const legacy = { permissions: [P.financeRefunds] }
    expect(hasCapability(legacy, P.financeRefunds, 'action')).toBe(false)
  })

  it('выдача read не даёт кнопку изменения', () => {
    const r = { grants: [{ capability: P.financeSales, level: 'read' as const }] }
    expect(hasCapability(r, P.financeSales, 'read')).toBe(true)
    expect(hasCapability(r, P.financeSales, 'write')).toBe(false)
  })

  it('без пользователя всё закрыто', () => {
    expect(hasCapability(null, P.staffView, 'read')).toBe(false)
    expect(hasPermission(null, P.staffView)).toBe(false)
  })
})