// Типы API v1 (STAIR PLATFORM). Снапшот результата хранится в экспортном
// документе с PascalCase-ключами (сериализация domain-типов); метаданные
// расчёта и проекты — snake_case (API-0015).

// DOM-001 (forensic 2026-09-24): тип марша — закрытый набор на бэкенде
// (engineering.FlightType.Valid()). Строковый тип позволял отправить
// «diagonal» и получить 500; union отражает контракт на границе.
export type FlightType = 'straight' | 'l_shape' | 'u_shape' | 'spiral'

export interface Project {
  id: string
  name: string
  description: string
  status: string
  owner_id: string
  current_configuration_id?: string
  created_at: string
  updated_at: string
}

// ---- Совместная работа (Phase C, EDR-0008): участники проекта ----

export type ProjectRole = 'owner' | 'editor' | 'viewer'

export interface ProjectMember {
  project_id: string
  user_id: string
  role: ProjectRole
  created_at: string
}

export interface MemberRequest {
  user_id?: string
  email?: string
  role: ProjectRole
}

// ---- Обсуждение (Phase C, EDR-0009): комментарии к проекту ----

export interface ProjectComment {
  id: string
  project_id: string
  author_id: string
  body: string
  created_at: string
}

export interface CommentRequest {
  body: string
}

// ---- Ревью (Phase C, EDR-0010): переходы статуса ревью проекта ----

export type ProjectStatus =
  | 'draft'
  | 'in_review'
  | 'approved'
  | 'changes_requested'

export type ReviewDecision = 'requested' | 'approved' | 'changes_requested'

export interface ProjectReview {
  id: string
  project_id: string
  requester_id: string
  reviewer_id: string
  decision: ReviewDecision
  comment: string
  created_at: string
  decided_at: string | null
}

export interface ReviewRequest {
  comment?: string
}

// ---- Утверждение конфигурации (EDR-0011): фиксация ревизии ----

export interface ConfigurationApproval {
  id: string
  project_id: string
  configuration_id: string
  approved_by: string
  comment: string
  created_at: string
}

export interface ApprovalRequest {
  comment?: string
}

// ---- Версионирование конфигурации (EDR-0012): иммутабельные ревизии ----

export interface Configuration {
  id: string
  project_id: string
  revision: number
  width_mm: number
  height_mm: number
  flight: FlightType
  step_height_mm: number
  stringer_thickness_mm: number
  step_thickness_mm: number
  riser: boolean
  clearance_mm: number
  railing_height_mm: number
  comfort_step_mm: number
  landing_width_mm: number
  lower_step_count: number
  outer_radius_mm: number
  current: boolean
  created_at: string
}

export interface User {
  id: string
  email: string
  name: string
  role: string
  tenant_id: string
}

// ---- Enterprise admin (Phase G, EDR-0016) ----

export interface AdminUser extends User {
  status: 'active' | 'disabled'
}

export interface AdminPolicy {
  min_password_length: number
  require_number: boolean
  require_upper: boolean
  session_ttl_seconds: number
  login_rate_limit_per_min: number
}

export interface AdminOverview {
  tenant_id: string
  users: number
  active_users: number
  disabled_users: number
  admins: number
  projects: number
  active_api_keys: number
  total_api_keys: number
}

export interface ApiKey {
  id: string
  name: string
  scopes: string[]
  created_by?: string
  created_at: string
  revoked_at?: string
  last_used_at?: string
}

// ---- Аналитика (Phase F, EDR-0028): метрики использования ----

export type UsageGranularity = 'day' | 'week' | 'month'

export interface UsageTotals {
  users: number
  active_users: number
  projects: number
  calculations: number
  logins: number
  exports: number
  payments: number
}

export interface UsagePoint {
  bucket: string
  logins: number
  active_users: number
  projects_created: number
  calculations: number
  exports: number
  payments: number
}

export interface UsageReport {
  from: string
  to: string
  granularity: UsageGranularity
  totals: UsageTotals
  series: UsagePoint[]
}

export interface ProjectRow {
  id: string
  name: string
  status: string
  owner_email: string
  created_at: string
  updated_at: string
  configurations: number
  calculations: number
  latest_calculation_valid: boolean | null
  comments: number
  members: number
}

export interface ProjectTotals {
  projects: number
  projects_created: number
  by_status: Record<string, number>
  projects_with_calculation: number
  valid_projects: number
  configurations: number
  calculations: number
  comments: number
}

export interface ProjectReport {
  from: string
  to: string
  totals: ProjectTotals
  projects: ProjectRow[]
}

export interface ManufacturingPoint {
  bucket: string
  calculations: number
  parts: number
  sheets: number
  utilization: number
}

export interface ManufacturingTotals {
  calculations: number
  parts: number
  bom_lines: number
  cut_items: number
  sheets: number
  part_area: number
  sheet_area: number
  waste_area: number
  utilization: number
  materials: Record<string, number>
}

export interface ManufacturingReport {
  from: string
  to: string
  granularity: UsageGranularity
  totals: ManufacturingTotals
  series: ManufacturingPoint[]
}

export interface CostPoint {
  bucket: string
  calculations: number
  final_price: number
  avg_final_price: number
}

export interface CostTotals {
  calculations: number
  material: number
  machine: number
  labor: number
  overhead: number
  production_cost: number
  margin: number
  discount: number
  pre_tax: number
  tax: number
  final_price: number
  avg_final_price: number
  currency: string
}

export interface CostReport {
  from: string
  to: string
  granularity: UsageGranularity
  totals: CostTotals
  series: CostPoint[]
}

// ---- Аудит (Phase G, EDR-0013): журнал событий безопасности ----

export interface AuditEvent {
  id: string
  actor_id?: string
  tenant_id: string
  project_id?: string
  action: string
  resource_type?: string
  resource_id?: string
  result: 'ok' | 'denied' | 'failed'
  detail?: string
  request_id?: string
  ip?: string
  created_at: string
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

export interface SnapshotSuggestion {
  StepCount: number
  LowerStepCount: number
  StepHeightMm: number
  TreadDepthMm: number
  AngleDeg: number
}

// Variation — альтернативная конфигурация (вариант A/B/C), устраняющая
// неблокирующее нарушение (напр. невписываемость в помещение). Config —
// переопределение полей формы (ключи как в ConfigForm, значения — строки);
// фронтенд сливает их в текущий конфиг и пересчитывает (превью).
export interface Variation {
  id: string
  title: string
  description: string
  config: Record<string, string>
  fits: boolean
  summary: string
}

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
  Param?: string
  Guide?: string
  Suggestions?: SnapshotSuggestion[]
  Variations?: Variation[]
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
  StepThickness?: number
  RailingHeight?: number
  Riser?: boolean
  StringerThickness?: number
  // Габариты помещения для прижима лестницы к стене (EDR-0023, room_fit).
  // Для прямого марша задаются явно (как у L-образной), чтобы при визуализации
  // ребро 1В можно было прижать к дальней стене В.
  RoomWidth?: number
  RoomLength?: number
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
  LandingDepth?: number
  RoomWidth?: number
  RoomLength?: number
  // Направление поворота (CONF-DIRECTION): 'left' | 'right' (план зеркалится).
  direction?: 'left' | 'right'
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
  // Направление поворота (CONF-DIRECTION): 'left' | 'right' (план зеркалится).
  direction?: 'left' | 'right'
}

export interface SpiralResult {
  StepCount: number
  StepHeight: number
  OuterRadius: number
  ColumnRadius: number
  WalkRadius: number
  InnerTread: number
  WalkTread: number
  OuterTread: number
  Angle: number
  AngularStep: number
  ArcLength: number
  ComfortStep: number
  AngularTotal: number
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
  spiral?: SpiralResult
  step_thickness?: number
  railing_height?: number
  riser?: boolean
  stringer_thickness?: number
  // Стороны перил из конфигурации (CONF-RAILING) — для 2D-рендера:
  // прямой/спираль — railing; L/U — по сегментам.
  railing?: string
  railing_lower?: string
  railing_landing?: string
  railing_upper?: string
  measurement?: Measurement
  mesh?: Mesh
  railing_mesh?: Mesh
  room_mesh?: Mesh
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

// OptimizeBest — лучшая конфигурация из оптимизации (EDR-0032).
export interface OptimizeBest {
  step_count: number
  step_height_mm: number
  tread_depth_mm: number
  lower_step_count?: number
  comfort_step_mm: number
  result: Snapshot
}

// OptimizeResult — итог оптимизации конфигурации (EDR-0032).
export interface OptimizeResult {
  valid: boolean
  evaluated: number
  target: string
  objective: number
  best?: OptimizeBest
  saved: boolean
  calculation_id?: string
  configuration_id?: string
}

// OptimizeTarget — целевая метрика оптимизации.
export type OptimizeTarget = 'price' | 'cost' | 'material'

// ---- AI-ассистенты (Phase D, EDR-0036): ответ и входные параметры ----

export type AssistantKind = 'design' | 'engineering' | 'manufacturing' | 'pricing'

export type AssistantPriority = 'price' | 'cost' | 'material' | 'comfort'

export interface AssistantFinding {
  severity: 'info' | 'warning' | 'error'
  element?: string
  message: string
}

export interface AssistantSuggestion {
  message: string
  rationale?: string
}

export interface AssistantAlternative {
  title: string
  rating: number
  reason?: string
}

export interface AssistantResponse {
  recommendation: string
  rating: number
  alternatives?: AssistantAlternative[]
  findings?: AssistantFinding[]
  suggestions?: AssistantSuggestion[]
  tradeoffs?: string[]
  notes?: string[]
}

export interface AssistantResult {
  kind: AssistantKind
  response: AssistantResponse
  commentary: string
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

// ---- Public Quote (store): POST /api/v1/public/stairs:quote ----
// Публичный результат расчёта для розничного клиента. Поля в snake_case
// соответствуют DTO бэкенда (public.go); без производственного пакета.

// QuoteSuggestion — готовый вариант конфигурации, проходящий все нормы
// (советник advisor). По нему фронтенд подставляет значения и повторяет расчёт.
export interface QuoteSuggestion {
  step_count: number
  lower_step_count?: number
  step_height_mm: number
  tread_depth_mm: number
  angle_deg: number
  // Для спирали (EDR-0007): наружный радиус и ширина марша варианта.
  outer_radius_mm?: number
  width_mm?: number
}

export interface QuoteIssue {
  code: string
  severity: string
  element: string
  message: string
  fix?: string
  param?: string
  guide?: string
  suggestions?: QuoteSuggestion[]
  variations?: Variation[]
}

export interface QuoteValidation {
  valid: boolean
  blocking: boolean
  issues: QuoteIssue[]
}

export interface QuoteFlight {
  step_count: number
  step_height_mm: number
  tread_depth_mm: number
  run_mm: number
  stringer_mm: number
  angle_deg: number
  width_mm: number
  step_thickness_mm: number
  railing_height_mm: number
  riser: boolean
  stringer_thickness_mm: number
  // Сторона перил (CONF-RAILING): none|left|right|both.
  railing?: string
  // Габариты помещения для прижима лестницы к стене (EDR-0023).
  // Для прямого марша задаются явно, чтобы визуализировать прижатие 1В→В.
  room_width_mm?: number
  room_length_mm?: number
}

export interface QuoteLShape {
  step_count: number
  lower_step_count: number
  upper_step_count: number
  step_height_mm: number
  tread_depth_mm: number
  angle_deg: number
  lower_height_mm: number
  upper_height_mm: number
  lower_run_mm: number
  upper_run_mm: number
  lower_stringer_mm: number
  upper_stringer_mm: number
  landing_width_mm: number
  landing_depth_mm?: number
  room_width_mm?: number
  room_length_mm?: number
  width_mm: number
  step_thickness_mm: number
  railing_height_mm: number
  riser: boolean
  stringer_thickness_mm: number
  // Перила по сегментам (CONF-RAILING) и поворот площадки (CONF-DIRECTION).
  railing_lower?: string
  railing_landing?: string
  railing_upper?: string
  direction?: string
}

export interface QuoteUShape {
  step_count: number
  lower_step_count: number
  upper_step_count: number
  step_height_mm: number
  tread_depth_mm: number
  angle_deg: number
  lower_height_mm: number
  upper_height_mm: number
  lower_run_mm: number
  upper_run_mm: number
  lower_stringer_mm: number
  upper_stringer_mm: number
  landing_width_mm: number
  width_mm: number
  step_thickness_mm: number
  railing_height_mm: number
  riser: boolean
  stringer_thickness_mm: number
  // Перила по сегментам (CONF-RAILING) и поворот площадки (CONF-DIRECTION).
  railing_lower?: string
  railing_landing?: string
  railing_upper?: string
  direction?: string
}

export interface QuoteSpiral {
  step_count: number
  step_height_mm: number
  outer_radius_mm: number
  column_radius_mm: number
  walk_radius_mm: number
  inner_tread_mm: number
  walk_tread_mm: number
  outer_tread_mm: number
  angle_deg: number
  angular_step_deg: number
  arc_length_mm: number
  comfort_step_mm: number
  angular_total_deg: number
  width_mm: number
  step_thickness_mm: number
  railing_height_mm: number
  riser: boolean
  stringer_thickness_mm: number
  // Перила (авто из направления, CONF-SPIRAL-RAILING) и направление закрутки.
  railing?: string
  spiral_direction?: string
}

export interface QuotePoint {
  x: number
  y: number
  z: number
}

export interface QuoteBBox {
  min: QuotePoint
  max: QuotePoint
}

export interface QuoteGeometry {
  solid_count: number
  volume_mm3: number
  surface_area_mm2: number
  bbox: QuoteBBox
}

export interface QuoteCostLine {
  name: string
  category: string
  amount_rub: number
  source: string
}

export interface QuotePricing {
  currency: string
  material_rub: number
  machine_rub: number
  labor_rub: number
  overhead_rub: number
  production_cost_rub: number
  margin_rub: number
  discount_rub: number
  pre_tax_rub: number
  tax_rub: number
  final_price_rub: number
  lines: QuoteCostLine[]
}

export interface QuoteResult {
  validation: QuoteValidation
  flight?: QuoteFlight
  lshape?: QuoteLShape
  ushape?: QuoteUShape
  spiral?: QuoteSpiral
  geometry?: QuoteGeometry
  pricing?: QuotePricing
  mesh?: Mesh
  railing_mesh?: Mesh
  room_mesh?: Mesh
}

// ---- Order (store/админка): /api/v1/orders ----

export interface OrderContact {
  name: string
  email: string
  phone?: string
}

export type OrderStatus =
  | 'new'
  | 'priced'
  | 'confirmed'
  | 'in_progress'
  | 'completed'
  | 'cancelled'

export type OrderKind = 'order' | 'consultation'

export interface OrderDTO {
  id: string
  kind: OrderKind
  status: OrderStatus
  contact: OrderContact
  config: unknown
  price?: unknown
  project_id?: string
  created_at: string
  updated_at: string
}

export interface CreateOrderRequest {
  contact: OrderContact
  config: unknown
  price: unknown
}

// ---- Отзывы клиентов (store): /api/v1/public/testimonials, /admin/testimonials ----

export interface TestimonialDTO {
  id: string
  author: string
  text: string
  rating: number
  published: boolean
  created_at: string
}

export interface CreateTestimonialRequest {
  author: string
  text: string
  rating: number
  published?: boolean
}

// Запрос консультации с лендинга (анонимный).
export interface CreateConsultationRequest {
  contact: OrderContact
  question: string
}
