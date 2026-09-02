package engineering

import (
	"fmt"
	"sync"
)

// EngineeringState — жизненный цикл инженерного объекта (EDM-0011).
type EngineeringState string

const (
	StateCreated    EngineeringState = "created"
	StateDraft      EngineeringState = "draft"
	StateEditing    EngineeringState = "editing"
	StateCalculated EngineeringState = "calculated"
	StateValidated  EngineeringState = "validated"
	StateApproved   EngineeringState = "approved"
	StateReleased   EngineeringState = "released"
	StateArchived   EngineeringState = "archived"
)

// ObjectKind — классификация инженерного объекта (EDM-0005).
type ObjectKind string

const (
	KindProject      ObjectKind = "project"
	KindAssembly     ObjectKind = "assembly"
	KindPart         ObjectKind = "part"
	KindGeometry     ObjectKind = "geometry"
	KindMaterial     ObjectKind = "material"
	KindFeature      ObjectKind = "feature"
	KindSketch       ObjectKind = "sketch"
	KindDrawing      ObjectKind = "drawing"
	KindQuotation    ObjectKind = "quotation"
	KindRevision     ObjectKind = "revision"
	KindInstallation ObjectKind = "installation"
	KindMaintenance  ObjectKind = "maintenance"
)

// DomainEvent — неизменяемое доменное событие (DOM-0007).
// Расширенный envelope для event-driven архитектуры (ARCH-0012).
type DomainEvent struct {
	Type          string
	ObjectID      string
	Revision      string
	Timestamp     string
	CorrelationID string // цепочка вызовов (трассировка через pipeline)
	Source        string // какой engine/domain создал событие
	Actor         string // кто инициировал действие
}

// EngineeringObject — корень Engineering Domain Model (ADR-0015).
// Каждый объект имеет ID, ревизию, жизненный цикл, параметры,
// ограничения, метаданные, события и историю.
type EngineeringObject struct {
	mu            sync.RWMutex
	ID            string
	Kind          ObjectKind
	State         EngineeringState
	Owner         string
	Current       *Revision
	Params        map[string]*Parameter
	Metadata      map[string]string
	events        []DomainEvent
	history       []*Revision
	correlationID string // трассировка pipeline (устанавливается извне)
}

// NewObject создаёт инженерный объект со стартовой ревизией.
func NewObject(id string, kind ObjectKind, owner, reason string) (*EngineeringObject, error) {
	if id == "" {
		return nil, fmt.Errorf("object: id is required")
	}
	if owner == "" {
		return nil, fmt.Errorf("object: owner is required")
	}
	rev, err := NewRevision(owner, reason, id)
	if err != nil {
		return nil, err
	}
	return &EngineeringObject{
		ID:       id,
		Kind:     kind,
		State:    StateCreated,
		Owner:    owner,
		Current:  rev,
		Params:   make(map[string]*Parameter),
		Metadata: make(map[string]string),
		events:   []DomainEvent{},
		history:  []*Revision{rev},
	}, nil
}

// AddParameter добавляет параметр и фиксирует событие.
func (o *EngineeringObject) AddParameter(p *Parameter) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, exists := o.Params[p.ID]; exists {
		return fmt.Errorf("object %s: parameter %s already exists", o.ID, p.ID)
	}
	o.Params[p.ID] = p
	o.record("parameter.added", p.ID)
	return nil
}

// Parameter возвращает параметр по ID.
func (o *EngineeringObject) Parameter(id string) (*Parameter, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	p, ok := o.Params[id]
	return p, ok
}

// Commit создаёт новую ревизию после изменения (неизменяемость истории).
func (o *EngineeringObject) Commit(author, reason string) (*Revision, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	next, err := o.Current.Child(author, reason, o.snapshotForChecksum())
	if err != nil {
		return nil, err
	}
	o.Current = next
	o.history = append(o.history, next)
	o.State = StateEditing
	o.record("revision.created", next.ID)
	return next, nil
}

// Transition выполняет допустимый переход состояния объекта.
func (o *EngineeringObject) Transition(target EngineeringState) error {
	allowed := map[EngineeringState]map[EngineeringState]bool{
		StateCreated:    {StateDraft: true},
		StateDraft:      {StateEditing: true},
		StateEditing:    {StateCalculated: true},
		StateCalculated: {StateValidated: true},
		StateValidated:  {StateApproved: true},
		StateApproved:   {StateReleased: true},
		StateReleased:   {StateArchived: true},
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if !allowed[o.State][target] {
		return fmt.Errorf("object %s: transition %s -> %s is not allowed", o.ID, o.State, target)
	}
	o.State = target
	o.record("object.state_changed", string(target))
	return nil
}

// Events возвращает неизменяемую копию истории событий.
func (o *EngineeringObject) Events() []DomainEvent {
	o.mu.RLock()
	defer o.mu.RUnlock()
	out := make([]DomainEvent, len(o.events))
	copy(out, o.events)
	return out
}

// History возвращает неизменяемую историю ревизий.
func (o *EngineeringObject) History() []*Revision {
	o.mu.RLock()
	defer o.mu.RUnlock()
	out := make([]*Revision, len(o.history))
	copy(out, o.history)
	return out
}

// SetCorrelationID устанавливает correlation ID для трассировки pipeline.
// Используется application layer при запуске pipeline расчёта.
func (o *EngineeringObject) SetCorrelationID(id string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.correlationID = id
}

// CorrelationID возвращает текущий correlation ID.
func (o *EngineeringObject) CorrelationID() string {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.correlationID
}

func (o *EngineeringObject) record(typ, objectID string) {
	o.events = append(o.events, DomainEvent{
		Type:          typ,
		ObjectID:      objectID,
		Revision:      o.Current.ID,
		CorrelationID: o.correlationID,
		Source:        "engineering",
		Actor:         o.Owner,
	})
}

func (o *EngineeringObject) snapshotForChecksum() string {
	return o.ID + "|" + string(o.Kind) + "|" + o.Current.ID
}
