import { describe, expect, it } from 'vitest'

import { notoSansBase64 } from './notoSans'

describe('notoSansBase64', () => {
  it('загружен и пригоден для jsPDF', () => {
    // Файл весит ~750 КБ и вставляется в PDF как addFileToVIPS. Если бы
    // шрифт отвалился от сборки, сломалась бы вёрстка КП — и молча, на
    // странице в браузере. Тест ловит пустой или обрезанный файл.
    expect(typeof notoSansBase64).toBe('string')
    expect(notoSansBase64.length).toBeGreaterThan(100_000)
  })

  it('начинается с сигнатуры TrueType', () => {
    // Основание base64 отражает первые четыре байта файла: 0x00010000 —
    // версия TrueType.
    expect(notoSansBase64.startsWith('AAEAAAASAQAABAA')).toBe(true)
  })

  it('содержит только символы base64', () => {
    expect(notoSansBase64).toMatch(/^[A-Za-z0-9+/=]+$/)
  })
})