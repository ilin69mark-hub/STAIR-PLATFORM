package engineering

import "fmt"

// Project — корневой агрегат (DOM-0002: Aggregate Root Project).
// Изменения проекта выполняются только через Root.
type Project struct {
	object *EngineeringObject
	name   string
}

// NewProject создаёт проект как агрегат с инженерным объектом.
func NewProject(id string, owner, name, reason string) (*Project, error) {
	if name == "" {
		return nil, fmt.Errorf("project: name is required")
	}
	obj, err := NewObject(id, KindProject, owner, reason)
	if err != nil {
		return nil, err
	}
	return &Project{object: obj, name: name}, nil
}

// ID возвращает идентификатор проекта.
func (p *Project) ID() string { return p.object.ID }

// Name возвращает имя проекта.
func (p *Project) Name() string { return p.name }

// Object возвращает инженерный объект проекта.
func (p *Project) Object() *EngineeringObject { return p.object }

// Rename изменяет имя проекта через Root.
func (p *Project) Rename(name string) error {
	if name == "" {
		return fmt.Errorf("project: name is required")
	}
	p.name = name
	return nil
}

// AddParameter добавляет параметр проекта.
func (p *Project) AddParameter(param *Parameter) error {
	return p.object.AddParameter(param)
}

// Commit создаёт новую ревизию проекта.
func (p *Project) Commit(author, reason string) (*Revision, error) {
	return p.object.Commit(author, reason)
}
