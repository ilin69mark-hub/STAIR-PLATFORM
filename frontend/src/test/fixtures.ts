// Общие фикстуры для тестов компонентов и снапшотов результата.
// Служат фейковым ответом API: валидный снапшот с flight/geometry/mfg/pricing.

import type {
  Calculation,
  FlightResult,
  Manufacturing,
  Measurement,
  Pricing,
  Project,
  Snapshot,
} from '../api/types'

export const flightFixture: FlightResult = {
  StepCount: 15,
  StepHeight: 180,
  TreadDepth: 280,
  Run: 4200,
  Stringer: 4783,
  Angle: Math.PI / 4,
}

export const measurementFixture: Measurement = {
  SolidCount: 20,
  Volume: 1950000000,
  SurfaceArea: 41000000,
  BoundingBox: {
    Min: { X: 0, Y: 0, Z: 0 },
    Max: { X: 4200, Y: 2700, Z: 900 },
  },
}

export const manufacturingFixture: Manufacturing = {
  Parts: [
    {
      Number: 'P1',
      Kind: 'stringer',
      Material: 'STEEL-S235',
      Thickness: 50,
      Length: 4783,
      Width: 300,
      SolidIndex: 1,
    },
  ],
  BOM: [
    {
      Number: 1,
      PartNumber: 'P1',
      Description: 'Косоур',
      Material: 'STEEL-S235',
      Thickness: 50,
      Quantity: 2,
      Length: 4783,
      Width: 300,
    },
  ],
  CutList: [],
  Nesting: {
    Sheets: [
      {
        Material: 'STEEL-S235',
        Thickness: 50,
        Length: 6000,
        Width: 1500,
        Placed: [{ PartNumber: 'P1', Length: 4783, Width: 300, X: 0, Y: 0 }],
      },
    ],
    PartCount: 1,
    PartArea: 1434900,
    SheetArea: 9000000,
    WasteArea: 7565100,
    Utilization: 0.16,
  },
}

export const pricingFixture: Pricing = {
  Currency: { Code: 'RUB', Decimals: 2 },
  Material: 100000,
  Machine: 20000,
  Labor: 30000,
  Overhead: 5000,
  ProductionCost: 155000,
  Margin: 30000,
  Discount: 0,
  PreTax: 185000,
  Tax: 37000,
  FinalPrice: 222000,
  Lines: [{ Name: 'Сталь', Category: 'material', Amount: 100000, Source: 'rate' }],
}

export function makeSnapshot(overrides: Partial<Snapshot> = {}): Snapshot {
  return {
    project_id: 'p1',
    validation: { Issues: [], Valid: true, Blocking: false },
    flight: flightFixture,
    measurement: measurementFixture,
    issue_count: 0,
    manufacturing: manufacturingFixture,
    pricing: pricingFixture,
    ...overrides,
  }
}

export function makeCalculation(overrides: Partial<Calculation> = {}): Calculation {
  return {
    project_id: 'p1',
    calculation_id: 'c1',
    configuration_id: 'k1',
    valid: true,
    blocking: false,
    created_at: '2026-08-14T10:00:00Z',
    result: makeSnapshot(),
    ...overrides,
  }
}

export function makeProject(overrides: Partial<Project> = {}): Project {
  return {
    id: 'p1',
    name: 'Лестница на второй этаж',
    description: '',
    status: 'active',
    created_at: '2026-08-14T10:00:00Z',
    updated_at: '2026-08-14T10:00:00Z',
    ...overrides,
  }
}