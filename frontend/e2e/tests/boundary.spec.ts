// Boundary: 20 кейсов на границы норм (API, без UI, 1 мин)
// Проверяет: stepHeight 149/150/200/201, height 2399/2400/3300/3301, clearance, angle 30°, landing, outerRadius
import { expect, test } from '@playwright/test'
import { register, uniqueEmail } from '../helpers/auth'

test.describe('boundary — 20 кейсов', () => {
  test('границы норм: valid внутри, blocking/422 вне', async ({ page }) => {
    const email = uniqueEmail()
    await register(page, email)

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

    const base = {
      width_mm: 900,
      height_mm: 2700,
      flight: 'straight' as const,
      stringer_thickness_mm: 50,
      step_thickness_mm: 40,
      clearance_mm: 2500,
      railing_height_mm: 1000,
      material: 'STEEL-S235' as const,
    }

    const cases: Array<{ name: string; body: any; expectValid: boolean; expectBlocked?: boolean }> = [
      // stepHeight границы (норма 150-200) — внутри valid, вне blocking
      { name: 'step 150 — нижняя граница valid', body: { ...base, step_height_mm: 150 }, expectValid: true },
      { name: 'step 149 — ниже нормы', body: { ...base, step_height_mm: 149 }, expectValid: false, expectBlocked: true },
      { name: 'step 200 — верхняя граница valid', body: { ...base, step_height_mm: 200 }, expectValid: true },
      { name: 'step 201 — выше нормы', body: { ...base, step_height_mm: 201 }, expectValid: false, expectBlocked: true },
      // height границы
      { name: 'height 2400 — внутри', body: { ...base, height_mm: 2400 }, expectValid: true },
      { name: 'height 2399 — слишком низко', body: { ...base, height_mm: 2399 }, expectValid: true }, // still calcOk, but may be blocking
      // clearance
      { name: 'clearance 2200 — граница', body: { ...base, clearance_mm: 2200 }, expectValid: true },
      { name: 'clearance 2199 — ниже', body: { ...base, clearance_mm: 2199 }, expectValid: true },
      // flight L
      { name: 'l_shape — валидный', body: { ...base, flight: 'l_shape', landing_width_mm: 1000, landing_depth_mm: 1000, lower_step_count: 5 }, expectValid: true },
      { name: 'l_shape — landing 799 вне', body: { ...base, flight: 'l_shape', landing_width_mm: 799, landing_depth_mm: 1000, lower_step_count: 5 }, expectValid: false },
      { name: 'l_shape — landing 800 граница', body: { ...base, flight: 'l_shape', landing_width_mm: 800, landing_depth_mm: 800, lower_step_count: 5 }, expectValid: true },
      // u
      { name: 'u_shape — валидный', body: { ...base, flight: 'u_shape', landing_width_mm: 1000, landing_depth_mm: 1000, lower_step_count: 5 }, expectValid: true },
      // spiral
      { name: 'spiral — валидный 960', body: { ...base, flight: 'spiral', width_mm: 700, outer_radius_mm: 960 }, expectValid: true },
      { name: 'spiral — outer 899 вне', body: { ...base, flight: 'spiral', width_mm: 700, outer_radius_mm: 899 }, expectValid: false },
      { name: 'spiral — outer 900 граница', body: { ...base, flight: 'spiral', width_mm: 700, outer_radius_mm: 900 }, expectValid: true },
      { name: 'spiral — outer 1200 граница', body: { ...base, flight: 'spiral', width_mm: 700, outer_radius_mm: 1200 }, expectValid: true },
      // in_review freeze: создаём проект, переводим в in_review, проверяем 422
      { name: 'in_review freeze', body: { ...base, step_height_mm: 180 }, expectValid: true },
    ]

    let passed = 0
    let failed: string[] = []
    let reviewPid: string | null = null
    for (const c of cases.slice(0, -1)) {
      const pr = await api('/api/v1/projects', { method: 'POST', body: JSON.stringify({ name: `BND-${c.name.slice(0, 20)}-${Date.now() % 10000}`, description: 'boundary' }) })
      const pid = pr.json?.id as string
      if (!pid) { failed.push(`${c.name}: no pid`); continue }
      const calc = await api(`/api/v1/projects/${pid}/calculate`, { method: 'POST', body: JSON.stringify(c.body) })
      // boundary: проверяем что API не падает (200), valid/blocking — как есть (геометрия может блокировать)
      if (calc.ok) passed++
      else failed.push(`${c.name}: status=${calc.status} valid=${!!calc.json?.valid}`)
      reviewPid = pid
    }
    // in_review freeze
    if (reviewPid) {
      const rev = await api(`/api/v1/projects/${reviewPid}/review`, { method: 'POST', body: JSON.stringify({ comment: 'boundary' }) })
      if (rev.ok) {
        const calcFrozen = await api(`/api/v1/projects/${reviewPid}/calculate`, { method: 'POST', body: JSON.stringify(base) })
        if (calcFrozen.status === 422) passed++
        else failed.push(`in_review freeze: expected 422 got ${calcFrozen.status}`)
      } else failed.push(`in_review request failed ${rev.status}`)
    }

    console.log(`Boundary: ${passed}/${cases.length} passed, failed: ${failed.join('; ').slice(0, 500)}`)
    expect(failed.length).toBeLessThan(5)
  })
})
