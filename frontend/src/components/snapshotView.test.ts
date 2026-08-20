import { describe, expect, it } from 'vitest'
import { schematicOf } from './snapshotView'
import type { Snapshot } from '@shared/types'

const snap = (over: Partial<Snapshot> = {}): Snapshot => ({
  project_id: 'p-1',
  validation: { Issues: [], Valid: true, Blocking: false },
  issue_count: 0,
  ...over,
})

describe('schematicOf railing (CONF-RAILING)', () => {
  it('прямой марш несёт сторону перил', () => {
    const s = schematicOf(
      snap({
        railing: 'none',
        flight: {
          StepCount: 15,
          StepHeight: 180,
          TreadDepth: 270,
          Run: 4050,
          Stringer: 4867,
          Angle: 0.58,
          RailingHeight: 900,
        },
      }),
    )
    expect(s?.kind).toBe('straight')
    expect(s?.railing).toBe('none')
  })

  it('L/П-образный: none только когда все сегменты без перил', () => {
    const base = {
      StepCount: 12,
      StepHeight: 180,
      TreadDepth: 270,
      Angle: 0.58,
      LowerHeight: 1080,
      UpperHeight: 2160,
      LowerRun: 1620,
      UpperRun: 1620,
      LowerStringer: 1942.8,
      UpperStringer: 1942.8,
      LandingWidth: 900,
    }
    const mk = (railing: Partial<Pick<Snapshot, 'railing_lower' | 'railing_landing' | 'railing_upper'>>) =>
      snap({ lshape: { ...base, LowerStepCount: 6, UpperStepCount: 6 }, ...railing })
    const allNone = schematicOf(
      mk({ railing_lower: 'none', railing_landing: 'none', railing_upper: 'none' }),
    )
    expect(allNone?.kind).toBe('l_shape')
    expect(allNone?.railing).toBe('none')

    const mixed = schematicOf(mk({ railing_lower: 'none', railing_landing: 'both', railing_upper: 'none' }))
    expect(mixed?.railing).toBeUndefined()
  })

  it('спираль несёт сторону перил (авто из направления)', () => {
    const s = schematicOf(
      snap({
        railing: 'left',
        spiral: {
          StepCount: 14,
          StepHeight: 192.8,
          OuterRadius: 800,
          ColumnRadius: 80,
          WalkRadius: 440,
          InnerTread: 100,
          WalkTread: 300,
          OuterTread: 500,
          Angle: 0.5,
          AngularStep: 0.4,
          ArcLength: 7037,
          ComfortStep: 685,
          AngularTotal: 6.28,
        },
      }),
    )
    expect(s?.kind).toBe('spiral')
    expect(s?.railing).toBe('left')
  })

  it('нулевой flight реального API не съедает тип L/U/спирали (REGB-01)', () => {
    const zeroFlight: Snapshot['flight'] = { StepCount: 0, StepHeight: 0, TreadDepth: 0, Run: 0, Stringer: 0, Angle: 0 }
    const l = schematicOf(
      snap({
        flight: zeroFlight,
        lshape: {
          StepCount: 12,
          StepHeight: 180,
          TreadDepth: 270,
          Angle: 0.58,
          LowerHeight: 1080,
          UpperHeight: 2160,
          LowerRun: 1620,
          UpperRun: 1620,
          LowerStringer: 1942.8,
          UpperStringer: 1942.8,
          LandingWidth: 900,
          LowerStepCount: 6,
          UpperStepCount: 6,
        },
      }),
    )
    expect(l?.kind).toBe('l_shape')
    expect(l?.flight.StepCount).toBe(12)

    const u = schematicOf(
      snap({
        flight: zeroFlight,
        ushape: {
          StepCount: 12,
          StepHeight: 180,
          TreadDepth: 270,
          Angle: 0.58,
          LowerHeight: 1080,
          UpperHeight: 2160,
          LowerRun: 1620,
          UpperRun: 1620,
          LowerStringer: 1942.8,
          UpperStringer: 1942.8,
          LandingWidth: 900,
          LowerStepCount: 6,
          UpperStepCount: 6,
        },
      }),
    )
    expect(u?.kind).toBe('u_shape')

    const sp = schematicOf(
      snap({
        flight: zeroFlight,
        spiral: {
          StepCount: 14,
          StepHeight: 192.8,
          OuterRadius: 800,
          ColumnRadius: 80,
          WalkRadius: 440,
          InnerTread: 100,
          WalkTread: 300,
          OuterTread: 500,
          Angle: 0.5,
          AngularStep: 0.4,
          ArcLength: 7037,
          ComfortStep: 685,
          AngularTotal: 6.28,
        },
      }),
    )
    expect(sp?.kind).toBe('spiral')
  })
})