// Typed domain events — конкретные типы событий по доменным контекстам (DOM-0007).
// Каждое событие содержит только минимально необходимые данные.
package events

// ─── Project Events (BC-001) ────────────────────────────────────────────────

const (
	EventProjectCreated  EventType = "project.created"
	EventProjectUpdated  EventType = "project.updated"
	EventProjectArchived EventType = "project.archived"
)

type ProjectCreated struct {
	BaseEvent
	ProjectID string
	Owner     string
	Name      string
}

type ProjectUpdated struct {
	BaseEvent
	ProjectID string
	FieldName string
}

type ProjectArchived struct {
	BaseEvent
	ProjectID string
}

// ─── Geometry Events (BC-002) ───────────────────────────────────────────────

const (
	EventGeometryUpdated   EventType = "geometry.updated"
	EventGeometryValidated EventType = "geometry.validated"
	EventFlightAdded       EventType = "geometry.flight_added"
	EventStepAdded         EventType = "geometry.step_added"
)

type GeometryUpdated struct {
	BaseEvent
	GeometryID string
	FlightType string
}

type GeometryValidated struct {
	BaseEvent
	GeometryID string
	Valid      bool
	IssueCount int
}

// ─── Validation Events (BC-004) ─────────────────────────────────────────────

const (
	EventValidationPassed EventType = "validation.passed"
	EventValidationFailed EventType = "validation.failed"
)

type ValidationPassed struct {
	BaseEvent
	ConfigID string
}

type ValidationFailed struct {
	BaseEvent
	ConfigID   string
	IssueCount int
	Blocking   bool
}

// ─── Solver Events (BC-003) ─────────────────────────────────────────────────

const (
	EventAnalysisStarted   EventType = "solver.analysis_started"
	EventAnalysisCompleted EventType = "solver.analysis_completed"
)

type AnalysisStarted struct {
	BaseEvent
	ConfigID string
	Flight   string
}

type AnalysisCompleted struct {
	BaseEvent
	ConfigID string
	Flight   string
	StepCount int
	Angle    float64
}

// ─── Optimization Events (BC-005) ───────────────────────────────────────────

const (
	EventBetterSolutionFound   EventType = "optimization.better_solution_found"
	EventOptimizationFinished  EventType = "optimization.finished"
)

type BetterSolutionFound struct {
	BaseEvent
	SolutionID string
	Score      float64
}

type OptimizationFinished struct {
	BaseEvent
	ConfigID    string
	CandidateCount int
	BestScore   float64
}

// ─── Manufacturing Events (BC-005) ──────────────────────────────────────────

const (
	EventBOMGenerated       EventType = "manufacturing.bom_generated"
	EventDrawingGenerated   EventType = "manufacturing.drawing_generated"
	EventPackageCompleted   EventType = "manufacturing.package_completed"
)

type BOMGenerated struct {
	BaseEvent
	ProjectID string
	PartCount int
	LineCount int
}

type PackageCompleted struct {
	BaseEvent
	ProjectID string
	PartCount int
	SheetCount int
}

// ─── Pricing Events (BC-006) ────────────────────────────────────────────────

const (
	EventPriceCalculated EventType = "pricing.price_calculated"
)

type PriceCalculated struct {
	BaseEvent
	ProjectID   string
	FinalPrice  int64
	Currency    string
}

// ─── Document Events (BC-009) ───────────────────────────────────────────────

const (
	EventDocumentGenerated EventType = "document.generated"
	EventDocumentPublished EventType = "document.published"
)

type DocumentGenerated struct {
	BaseEvent
	DocumentID string
	DocType    string
	ProjectID  string
}

type DocumentPublished struct {
	BaseEvent
	DocumentID string
	DocType    string
	ProjectID  string
}

// ─── Engine Events (меж-engine коммуникация, ENG-0005) ──────────────────────

const (
	EventEngineGeometryFinished     EventType = "engine.geometry_finished"
	EventEngineValidationFinished   EventType = "engine.validation_finished"
	EventEngineAnalysisFinished     EventType = "engine.analysis_finished"
	EventEngineOptimizationFinished EventType = "engine.optimization_finished"
	EventEngineManufacturingFinished EventType = "engine.manufacturing_finished"
	EventEnginePricingFinished      EventType = "engine.pricing_finished"
	EventEngineRenderingFinished    EventType = "engine.rendering_finished"
	EventEngineDocumentsFinished    EventType = "engine.documents_finished"
	EventPipelineCompleted          EventType = "engine.pipeline_completed"
	EventPipelineFailed             EventType = "engine.pipeline_failed"
)

type EngineFinished struct {
	BaseEvent
	Engine    string
	ConfigID  string
	Duration  float64 // milliseconds
	Success   bool
	ErrorMsg  string
}

type PipelineCompleted struct {
	BaseEvent
	ConfigID string
	Duration float64 // total pipeline duration in ms
}

type PipelineFailed struct {
	BaseEvent
	ConfigID string
	Stage    string
	ErrorMsg string
}

// ─── System Events ──────────────────────────────────────────────────────────

const (
	EventSystemStarted EventType = "system.started"
	EventSystemStopped EventType = "system.stopped"
)

type SystemStarted struct {
	BaseEvent
	Version string
}

type SystemStopped struct {
	BaseEvent
	Reason string
}
