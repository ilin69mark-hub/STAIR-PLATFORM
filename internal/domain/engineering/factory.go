package engineering

import "errors"

// Factory создаёт полностью валидные агрегаты (DOM-0011).
// Factory не содержит инфраструктуры, не обращается к Repository и SQL.
type Factory struct{}

var (
	errProjectIDRequired    = errors.New("factory: project id is required")
	errProjectOwnerRequired = errors.New("factory: project owner is required")
)

// NewFactory возвращает фабрику проектов.
func NewFactory() *Factory {
	return &Factory{}
}

// CreateProject создаёт полностью валидный проект со стартовой ревизией.
func (f *Factory) CreateProject(id, owner, name, reason string) (*Project, error) {
	if id == "" {
		return nil, errProjectIDRequired
	}
	if owner == "" {
		return nil, errProjectOwnerRequired
	}
	return NewProject(id, owner, name, reason)
}

// CreateStairConfiguration создаёт валидную конфигурацию лестницы.
func (f *Factory) CreateStairConfiguration(width, height Length, flight FlightType) (*StairConfiguration, error) {
	return NewStairConfiguration(width, height, flight)
}
