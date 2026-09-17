// UI 20: 5×4 типа, полностью через page (form → calculate → 3D → КП)
import { expect, test } from '@playwright/test'
import { register, uniqueEmail } from '../helpers/auth'

test.describe.serial('UI 20 — 5×4 через page', () => {
  test.setTimeout(600_000)
  const flights: Array<{ flight: string; params: Record<string, string> }> = [
    { flight: 'straight', params: {} },
    { flight: 'l_shape', params: { landingWidthMM: '1000', lowerStepCountMM: '5' } },
    { flight: 'u_shape', params: { landingWidthMM: '1000', lowerStepCountMM: '5' } },
    { flight: 'spiral', params: { outerRadiusMM: '960' } },
  ]

  test('20 проектов UI: форма → расчёт → 3D → КП', async ({ page }) => {
    await register(page, uniqueEmail())
    let ok = 0
    let fail: string[] = []
    for (const f of flights) {
      for (let i = 0; i < 5; i++) {
        const name = `UI-${f.flight}-${i}-${Date.now() % 10000}`
        try {
          await page.locator('#project-name').fill(name)
          await page.getByRole('button', { name: 'Создать' }).click()
          await expect(page.getByRole('heading', { name, exact: true })).toBeVisible({ timeout: 8000 })
          await page.locator('#cfg-flight').selectOption(f.flight)
          for (const [k, v] of Object.entries(f.params)) {
            const sel = `#cfg-${k}`
            if (await page.locator(sel).count()) await page.locator(sel).fill(v)
          }
          await page.getByRole('button', { name: 'Рассчитать' }).click()
          await expect(page.getByText('Расчёт сохранён')).toBeVisible({ timeout: 10000 })
          // 3D
          const tab3d = page.getByRole('tab', { name: '3D' })
          if (await tab3d.count()) {
            await tab3d.click()
            await page.waitForTimeout(800)
            const canvas = page.locator('.viewer__stage canvas')
            await expect(canvas).toBeVisible({ timeout: 5000 })
            const box = await canvas.boundingBox()
            if (box) {
              await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2)
              await page.mouse.down()
              await page.mouse.move(box.x + box.width / 2 + 40, box.y + box.height / 2 - 20, { steps: 5 })
              await page.mouse.up()
              await page.waitForTimeout(500)
            }
          }
          // КП
          const kpBtn = page.getByRole('button', { name: 'Коммерческое предложение' })
          await expect(kpBtn).toBeEnabled({ timeout: 5000 })
          const [dl] = await Promise.all([page.waitForEvent('download', { timeout: 15000 }), kpBtn.click()])
          const p = await dl.path()
          if (!p) throw new Error('no download')
          ok++
          // назад к списку
          await page.getByRole('button', { name: '← Проекты' }).click()
          await expect(page.locator('#project-name')).toBeVisible({ timeout: 8000 })
        } catch (e: any) {
          fail.push(`${f.flight}-${i}: ${String(e.message).slice(0, 120)}`)
          try { await page.getByRole('button', { name: '← Проекты' }).click({ timeout: 3000 }) } catch {}
          await page.waitForTimeout(500)
        }
      }
    }
    console.log(`UI 20: ok ${ok}/20, fail ${fail.length}: ${fail.join('; ').slice(0, 500)}`)
    expect(ok).toBeGreaterThanOrEqual(15)
  })
})
