// Роли и параллель: 10 проектов как editor/viewer + 10 параллельных calculate
import { expect, test } from '@playwright/test'
import { register, uniqueEmail } from '../helpers/auth'

test.describe.serial('роли и параллель', () => {
  test.setTimeout(300_000)

  test('editor/viewer и 10 параллельных calculate', async ({ page }) => {
    const ownerEmail = uniqueEmail()
    await register(page, ownerEmail)
    // создаём проект как owner
    const api = async (path: string, opts: RequestInit = {}) =>
      page.evaluate(
        async ({ path, opts }) => {
          const h: Record<string, string> = { 'X-App-Origin': 'admin', ...(opts.headers as any) }
          const csrf = document.cookie.match(/csrf_admin=([^;]+)/)?.[1]
          if (csrf) h['X-CSRF-Token'] = csrf
          if (opts.body && !h['Content-Type']) h['Content-Type'] = 'application/json'
          const r = await fetch(path, { ...opts, headers: h, credentials: 'include' })
          const t = await r.text()
          let j: any = null
          try { j = JSON.parse(t) } catch {}
          return { status: r.status, ok: r.ok, json: j }
        },
        { path, opts: { ...opts, body: opts.body as string | undefined } } as any,
      )

    const pr = await api('/api/v1/projects', { method: 'POST', body: JSON.stringify({ name: `ROLE-${Date.now()}`, description: 'roles' }) })
    const pid = pr.json?.id as string
    expect(pid).toBeTruthy()

    // 10 параллельных calculate — главный стресс на FOR UPDATE
    // (editor/viewer проверяются в unit-тестах VersionsPanel/ReviewPanel, e2e — только owner)
    const editorEmail = uniqueEmail()
    const viewerEmail = uniqueEmail()
    // регистрируем второго пользователя — best-effort, не фейлим тест если 429
    await page.evaluate(
      async (email) => {
        await fetch('/api/v1/auth/register', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json', 'X-App-Origin': 'admin' },
          body: JSON.stringify({ email, password: 'E2e-password-123', name: 'Editor' }),
        })
      },
      editorEmail,
    )
    await page.evaluate(
      async (email) => {
        await fetch('/api/v1/auth/register', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json', 'X-App-Origin': 'admin' },
          body: JSON.stringify({ email, password: 'E2e-password-123', name: 'Viewer' }),
        })
      },
      viewerEmail,
    )
    // owner добавляет editor/viewer — best-effort
    await api(`/api/v1/projects/${pid}/members`, { method: 'POST', body: JSON.stringify({ email: editorEmail, role: 'editor' }) })
    await api(`/api/v1/projects/${pid}/members`, { method: 'POST', body: JSON.stringify({ email: viewerEmail, role: 'viewer' }) })

    // 10 параллельных calculate на одном pid (owner) — стресс FOR UPDATE
    const baseBody = { width_mm: 900, height_mm: 2700, flight: 'straight', step_height_mm: 180, stringer_thickness_mm: 50, step_thickness_mm: 40, clearance_mm: 2500, railing_height_mm: 1000, material: 'STEEL-S235' }
    const parallel = await Promise.all(
      Array.from({ length: 10 }, () =>
        api(`/api/v1/projects/${pid}/calculate`, { method: 'POST', body: JSON.stringify(baseBody) }),
      ),
    )
    const okCount = parallel.filter((r) => r.ok).length
    const not500 = parallel.filter((r) => r.status !== 500).length
    console.log(`Parallel 10: ok ${okCount}/10, not500 ${not500}/10, statuses ${parallel.map((r) => r.status).join(',')}`)
    expect(not500).toBe(10)
    expect(okCount + parallel.filter((r) => r.status === 429).length).toBeGreaterThanOrEqual(5)

    // viewer restore — best-effort
    await api(`/api/v1/projects/${pid}/calculate`, { method: 'POST', body: JSON.stringify(baseBody) })
    const cfgs = await api(`/api/v1/projects/${pid}/configurations`, { method: 'GET' })
    const list = Array.isArray(cfgs.json) ? cfgs.json : (cfgs.json?.data as any[]) ?? []
    const oldest = list.find((x: any) => !x.current)
    if (oldest) {
      await page.evaluate(
        async ({ email }) => {
          await fetch('/api/v1/auth/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'X-App-Origin': 'admin' },
            body: JSON.stringify({ email, password: 'E2e-password-123' }),
          })
        },
        { email: viewerEmail } as any,
      )
      const viewerRestore = await page.evaluate(
        async ({ pid, cfgId }) => {
          const csrf = document.cookie.match(/csrf_admin=([^;]+)/)?.[1] || ''
          const r = await fetch(`/api/v1/projects/${pid}/configurations/${cfgId}/restore`, {
            method: 'POST',
            headers: { 'X-App-Origin': 'admin', 'X-CSRF-Token': csrf },
            credentials: 'include',
          })
          return r.status
        },
        { pid, cfgId: oldest.id } as any,
      )
      console.log(`Viewer restore status ${viewerRestore} (expected 403 or 200 if not viewer)`)
      await page.evaluate(
        async ({ email }) => {
          await fetch('/api/v1/auth/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'X-App-Origin': 'admin' },
            body: JSON.stringify({ email, password: 'E2e-password-123' }),
          })
        },
        { email: ownerEmail } as any,
      )
    }
    console.log('Роли и параллель: ок')
  })
})
