import { describe, expect, it } from 'vitest'
import { flightOptions, SPIRAL_ENABLED } from '@shared/config'

// Витрина обещает только те типы марша, которые реально считаются: список
// витрины и расчётных ручек API должны совпадать (S-152 — спираль отключена).
describe('витрина: доступные типы марша', () => {
  it('спираль скрыта вместе с отключением в API', () => {
    const values = flightOptions.map((o) => o.value)
    expect(SPIRAL_ENABLED).toBe(false)
    expect(values).not.toContain('spiral')
  })

  it('остальные три типа марша доступны', () => {
    expect(flightOptions.map((o) => o.value)).toEqual(['straight', 'l_shape', 'u_shape'])
  })
})
