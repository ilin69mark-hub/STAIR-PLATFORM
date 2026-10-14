// projectsApi — реальные HTTP-обёртки. Мокаем глобальный fetch (как в
// admin.test.ts) и проверяем URL, метод, тело и десериализацию.
//
// Смысл теста не в «строка не упала», а в адресе: ошибка в шаблоне пути даёт
// 404 в проде, и заметить её можно только на этапе пользователя.
import { afterEach, describe, expect, it, vi } from 'vitest'

import { projectsApi } from './projects'

function fetchMock() {
  const fn = vi.fn()
  vi.stubGlobal('fetch', fn)
  return fn
}

function jsonResponse(status: number, body?: string) {
  return {
    status,
    ok: status >= 200 && status < 300,
    text: vi.fn().mockResolvedValue(body ?? ''),
  }
}

function lastCall(fn: ReturnType<typeof fetchMock>) {
  const call = fn.mock.calls.at(-1)!
  return { url: call[0] as string, init: (call[1] ?? {}) as RequestInit }
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('projectsApi', () => {
  it('list шлёт GET /api/v1/projects', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({ items: [], total: 0 })))
    await projectsApi.list()
    expect(lastCall(fn).url).toBe('/api/v1/projects')
  })

  it('get разворачивает data ответа', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({ id: 'p-1', name: 'Лестница' })))
    const p = await projectsApi.get('p-1')
    expect(lastCall(fn).url).toBe('/api/v1/projects/p-1')
    expect(p.id).toBe('p-1')
  })

  it('create отправляет POST с телом', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(201, JSON.stringify({ id: 'p-2' })))
    await projectsApi.create({ name: 'Проект' } as never)
    const { url, init } = lastCall(fn)
    expect(url).toBe('/api/v1/projects')
    expect(init.method).toBe('POST')
    expect(init.body).toBe(JSON.stringify({ name: 'Проект' }))
  })

  it('listReviews разворачивает data', async () => {
    const fn = fetchMock()
    // Обёртка снимает .data: сервер отдаёт {data, page, total}, а наружу уходит
    // сам массив. Если бы .data забыли, вызывающий получил бы undefined вместо
    // списка — и упал бы уже в компоненте, вдали от причины.
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({ data: [{ id: 'r-1' }] })))
    const r = await projectsApi.listReviews('p-1')
    expect(lastCall(fn).url).toBe('/api/v1/projects/p-1/reviews')
    expect(r).toHaveLength(1)
  })

  it('listApprovals разворачивает data', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({ data: [{ id: 'ca-1' }] })))
    const r = await projectsApi.listApprovals('p-1')
    expect(lastCall(fn).url).toBe('/api/v1/projects/p-1/approvals')
    expect(r).toHaveLength(1)
  })

  it('listComments разворачивает data', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({ data: [{ id: 'c-1' }] })))
    const r = await projectsApi.listComments('p-1')
    expect(lastCall(fn).url).toBe('/api/v1/projects/p-1/comments')
    expect(r).toHaveLength(1)
  })

  it('calculate шлёт POST в проект', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({ id: 'c-1' })))
    await projectsApi.calculate('p-1', { steps: 16 })
    expect(lastCall(fn).url).toBe('/api/v1/projects/p-1/calculate')
  })

  it('validateStair идёт по пути без проекта', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({ validation: {} })))
    await projectsApi.validateStair({ steps: 16 })
    // Валидация не привязана к проекту: /projects/:id/stairs:validate был бы
    // 404, потому что такого маршрута нет.
    expect(lastCall(fn).url).toBe('/api/v1/stairs:validate')
  })

  it('preview и optimize различаются путём', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({})))
    await projectsApi.preview('p-1', {})
    expect(lastCall(fn).url).toBe('/api/v1/projects/p-1/preview')
    await projectsApi.optimize('p-1', {})
    expect(lastCall(fn).url).toBe('/api/v1/projects/p-1/optimize')
  })

  it('exportUrl не ходит в сеть', async () => {
    const fn = fetchMock()
    expect(projectsApi.exportUrl('p-9')).toBe('/api/v1/projects/p-9/export')
    expect(fn).not.toHaveBeenCalled()
  })

  it('updateMemberRole шлёт PATCH с новой ролью', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({ status: 'ok' })))
    await projectsApi.updateMemberRole('p-1', 'u-2', 'editor')
    const { url, init } = lastCall(fn)
    expect(url).toBe('/api/v1/projects/p-1/members/u-2')
    expect(init.method).toBe('PATCH')
    expect(init.body).toBe(JSON.stringify({ role: 'editor' }))
  })

  it('transferOwnership кладёт получателя в тело, а не в путь', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({ status: 'ok' })))
    await projectsApi.transferOwnership('p-1', 'u-9')
    const { url, init } = lastCall(fn)
    expect(url).toBe('/api/v1/projects/p-1/transfer')
    expect(init.body).toBe(JSON.stringify({ to_user_id: 'u-9' }))
  })

  it('markMeasurement по умолчанию ставит mark=true', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({ id: 'cfg-1' })))
    await projectsApi.markMeasurement('p-1', 'cfg-1')
    expect(lastCall(fn).init.body).toBe(JSON.stringify({ mark: true }))
    await projectsApi.markMeasurement('p-1', 'cfg-1', false)
    expect(lastCall(fn).init.body).toBe(JSON.stringify({ mark: false }))
  })

  it('assistant адресует вид ассистента из аргумента', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({ ok: true })))
    await projectsApi.assistant('pricing' as never, { q: 1 })
    // Вид ассистента подставляется в путь: опечатка здесь уводит запрос на
    // несуществующий маршрут и молча возвращает 404.
    expect(lastCall(fn).url).toBe('/api/v1/assistant/pricing')
  })

  it('approveConfiguration адресует конфигурацию внутри проекта', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({ id: 'ca-1' })))
    await projectsApi.approveConfiguration('p-1', 'cfg-1', {} as never)
    const { url, init } = lastCall(fn)
    expect(url).toBe('/api/v1/projects/p-1/configurations/cfg-1/approve')
    expect(init.method).toBe('POST')
  })

  it('getConfigurationApproval шлёт GET', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({ id: 'ca-1' })))
    await projectsApi.getConfigurationApproval('p-1', 'cfg-1')
    expect(lastCall(fn).url).toBe('/api/v1/projects/p-1/configurations/cfg-1/approval')
  })

  it('listConfigurations возвращает массив как есть', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify([{ id: 'cfg-1' }])))
    const list = await projectsApi.listConfigurations('p-1')
    expect(lastCall(fn).url).toBe('/api/v1/projects/p-1/configurations')
    expect(list).toHaveLength(1)
  })

  it('getConfiguration шлёт GET с идентификатором', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({ id: 'cfg-1' })))
    await projectsApi.getConfiguration('p-1', 'cfg-1')
    expect(lastCall(fn).url).toBe('/api/v1/projects/p-1/configurations/cfg-1')
  })

  it('restoreConfiguration шлёт POST с пустым телом', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({ id: 'cfg-1' })))
    await projectsApi.restoreConfiguration('p-1', 'cfg-1')
    const { url, init } = lastCall(fn)
    expect(url).toBe('/api/v1/projects/p-1/configurations/cfg-1/restore')
    expect(init.method).toBe('POST')
  })

  it('assistant адресует вид ассистента из аргумента', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({ ok: true })))
    await projectsApi.assistant('pricing' as never, { q: 1 })
    // Вид ассистента подставляется в путь: опечатка здесь уводит запрос на
    // несуществующий маршрут и молча возвращает 404.
    expect(lastCall(fn).url).toBe('/api/v1/assistant/pricing')
  })
})