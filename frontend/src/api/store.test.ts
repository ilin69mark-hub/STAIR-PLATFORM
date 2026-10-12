// storeApi — HTTP-обёртки витрины. Проверяем адрес, метод и тело: ошибка в
// шаблоне пути уводит PUT на другой ресурс и выглядит как «сохранилось».
import { afterEach, describe, expect, it, vi } from 'vitest'

import { storeApi } from './store'

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

describe('storeApi', () => {
  it('settings читает документ целиком', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({ contacts: {} })))
    await storeApi.settings()
    expect(lastCall(fn).url).toBe('/api/v1/admin/store/settings')
  })

  it('saveSettings шлёт PUT с телом', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({})))
    await storeApi.saveSettings({ contacts: {} } as never)
    const { url, init } = lastCall(fn)
    expect(url).toBe('/api/v1/admin/store/settings')
    expect(init.method).toBe('PUT')
  })

  it('saveSettingsGroup адресует группу в пути', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({})))
    await storeApi.saveSettingsGroup('seo', { title: 'x' } as never)
    // Без группы в пути «сохранить только SEO» означало бы «и реквизиты
    // тоже» — PUT перезаписывал бы весь документ.
    expect(lastCall(fn).url).toBe('/api/v1/admin/store/settings/seo')
  })

  it('цены материалов читаются и правятся', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify([])))
    await storeApi.prices()
    expect(lastCall(fn).url).toBe('/api/v1/admin/store/prices')

    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({})))
    await storeApi.setPrice('OAK', 120)
    const { init } = lastCall(fn)
    expect(init.method).toBe('PUT')
    expect(init.body).toBe(JSON.stringify({ code: 'OAK', price_per_kg_rub: 120 }))
  })

  it('коды материалов экранируются при сбросе', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(204))
    // Код вида «TUBE/50» без экранирования разорвал бы путь на два сегмента
    // и удалил бы не тот ресурс — молча, с кодом ответа 404.
    await storeApi.resetPrice('TUBE/50')
    expect(lastCall(fn).url).toBe('/api/v1/admin/store/prices/TUBE%2F50')
  })

  it('крепёж — отдельный ресурс, единица измерения другая', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify([])))
    await storeApi.fasteners()
    expect(lastCall(fn).url).toBe('/api/v1/admin/store/fasteners')

    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({})))
    await storeApi.setFastenerPrice('SCREW_M6', 15)
    // Рубли за штуку, а не за кг: поле price_per_pc_rub, не price_per_kg_rub.
    expect(lastCall(fn).init.body).toBe(
      JSON.stringify({ code: 'SCREW_M6', price_per_pc_rub: 15 }),
    )
  })

  it('герметик хранится в рублях за картридж', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify([])))
    await storeApi.sealants()
    expect(lastCall(fn).url).toBe('/api/v1/admin/store/sealants')

    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({})))
    await storeApi.setSealantPrice('SILICONE', 900)
    expect(lastCall(fn).init.body).toBe(
      JSON.stringify({ code: 'SILICONE', price_per_cartridge_rub: 900 }),
    )
  })

  it('публичные каталоги берутся без префикса admin', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify([])))
    await storeApi.materials()
    expect(lastCall(fn).url).toBe('/api/v1/public/materials')

    await storeApi.tiers()
    expect(lastCall(fn).url).toBe('/api/v1/public/payment-tiers')
  })

  it('платежи и возврат', async () => {
    const fn = fetchMock()
    fn.mockResolvedValue(jsonResponse(200, JSON.stringify([])))
    await storeApi.payments()
    expect(lastCall(fn).url).toBe('/api/v1/admin/payments')

    fn.mockResolvedValue(jsonResponse(200, JSON.stringify({})))
    await storeApi.refundPayment('pay-1')
    expect(lastCall(fn).url).toContain('pay-1')
  })
})