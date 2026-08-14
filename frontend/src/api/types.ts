// Типы API v1 (STAIR PLATFORM). Снапшот результата хранится в экспортном
// документе с PascalCase-ключами (сериализация domain-типов); метаданные
// расчёта и проекты — snake_case (API-0015).

export interface Project {
  id: string
  name: string
  description: string
  status: string
  created_at: string
  updated_at: string
}

export interface User {
  id: string
  email: string
  name: string
  role: string
  tenant_id: string
}

export interface CreateProjectRequest {
  name: string
  description: string
}

// ---- Ставки цены (PRC) — опциональное переопределение дефолтов ----

export interface Rates {
  material_per_kg_rub?: {
    'STEEL-S235'?: number
    'ALUM-5083'?: number
    'WOOD-OAK'?: number
  }
  machine_per_hour_rub?: number
  labor_per_hour_rub?: number
  overhead_percent?: number
  margin_percent?: number
  discount_percent?: number
  tax_percent?: number
}

// ---- Экспортный документ (снапшот) ----

export interface ValidationIssue {
  ID?: string
  Code: string
  Severity: string
  Element: string
  Message: string
  Value?: number
  Min?: number
  Max?: number
  Fix?: string
}

export interface ValidationResult {
  Issues: ValidationIssue[]
  Valid: boolean
  Blocking: boolean
}

export interface FlightResult {
  StepCount: number
  StepHeight: number
  TreadDepth: number
  Run: number
  Stringer: number
  Angle: number
}

export interface LShapeResult {
  StepCount: number
  LowerStepCount: number
  UpperStepCount: number
  StepHeight: number
  TreadDepth: number
  Angle: number
  LowerHeight: number
  UpperHeight: number
  LowerRun: number
  UpperRun: number
  LowerStringer: number
  UpperStringer: number
  LandingWidth: number
}

export interface UShapeResult {
  StepCount: number
  LowerStepCount: number
  UpperStepCount: number
  StepHeight: number
  TreadDepth: number
  Angle: number
  LowerHeight: number
  UpperHeight: number
  LowerRun: number
  UpperRun: number
  LowerStringer: number
  UpperStringer: number
  LandingWidth: number
}

export interface BBox {
  Min: { X: number; Y: number; Z: number }
  Max: { X: number; Y: number; Z: number }
}

export interface Measurement {
  SolidCount: number
  Volume: number
  SurfaceArea: number
  BoundingBox: BBox
}

export interface MeshVertex {
  X: number
  Y: number
  Z: number
}

export interface Mesh {
  Vertices: MeshVertex[]
  Triangles: Array<[number, number, number]>
}

export interface Part {
  Number: string
  Kind: string
  Material: string
  Thickness: number
  Length: number
  Width: number
  SolidIndex: number
}

export interface BomLine {
  Number: number
  PartNumber: string
  Description: string
  Material: string
  Thickness: number
  Quantity: number
  Length: number
  Width: number
}

export interface CutItem {
  PartNumber: string
  Material: string
  Thickness: number
  Length: number
  Width: number
  Quantity: number
}

export interface PlacedPart {
  PartNumber: string
  Length: number
  Width: number
  X: number
  Y: number
}

export interface SheetLayout {
  Material: string
  Thickness: number
  Length: number
  Width: number
  Placed: PlacedPart[]
}

export interface Nesting {
  Sheets: SheetLayout[]
  PartCount: number
  PartArea: number
  SheetArea: number
  WasteArea: number
  Utilization: number
}

export interface BomDocument {
  Lines: BomLine[]
}

export interface CutListDocument {
  Items: CutItem[]
}

export interface Manufacturing {
  Parts: Part[]
  BOM: BomDocument
  CutList: CutListDocument
  Nesting: Nesting
}

export interface CostComponent {
  Name: string
  Category: string
  Amount: number // минорные единицы (копейки)
  Source: string
}

export interface Pricing {
  Currency: { Code: string; Decimals: number }
  Material: number
  Machine: number
  Labor: number
  Overhead: number
  ProductionCost: number
  Margin: number
  Discount: number
  PreTax: number
  Tax: number
  FinalPrice: number
  Lines: CostComponent[]
}

export interface Snapshot {
  project_id: string
  validation: ValidationResult
  flight?: FlightResult
  lshape?: LShapeResult
  ushape?: UShapeResult
  measurement?: Measurement
  mesh?: Mesh
  issue_count: number
  manufacturing?: Manufacturing
  pricing?: Pricing
  cost?: Record<string, unknown>
}

export interface Calculation {
  project_id: string
  calculation_id: string
  configuration_id: string
  valid: boolean
  blocking: boolean
  created_at: string
  result: Snapshot
}

export interface ApiErrorBody {
  error: { code: string; message: string }
}

export class ApiError extends Error {
  code: string
  status: number

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
  }
}
