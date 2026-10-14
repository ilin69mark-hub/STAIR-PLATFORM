import { describe, expect, it } from 'vitest'
import { flightOptions } from '@shared/config'

// Витрина обещает только те типы марша, которые реально считаются: список
// витрины и расчётных ручек API должны совпадать.
describe('витрина: доступные типы марша', () => {
  it('ровно три типа марша', () => {
    expect(flightOptions.map((o) => o.value)).toEqual(['straight', 'l_shape', 'u_shape'])
  })
})
