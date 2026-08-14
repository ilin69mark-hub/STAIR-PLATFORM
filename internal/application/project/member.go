package project

import (
	"fmt"
	"time"
)

// ProjectRole — роль участника проекта (EDR-0008, Phase C).
// Определяет допустимые действия в рамках проекта.
type ProjectRole string

const (
	// RoleOwner — владелец проекта: полное управление, включая
	// управление членами и изменение проекта.
	RoleOwner ProjectRole = "owner"
	// RoleEditor — редактор: может изменять проект и конфигурацию.
	RoleEditor ProjectRole = "editor"
	// RoleViewer — наблюдатель: только чтение проекта.
	RoleViewer ProjectRole = "viewer"
)

// ProjectMember — член проекта (EDR-0008). Владелец — член с ролью
// RoleOwner; на проект приходится ровно один владелец (инвариант,
// соблюдается в Repository/SQL через уникальный частичный индекс).
type ProjectMember struct {
	ProjectID string
	UserID    string
	Role      ProjectRole
	CreatedAt time.Time
}

// ParseProjectRole нормализует роль из строки DTO.
func ParseProjectRole(s string) (ProjectRole, error) {
	switch ProjectRole(s) {
	case RoleOwner, RoleEditor, RoleViewer:
		return ProjectRole(s), nil
	default:
		return "", fmt.Errorf("project: unknown role %q", s)
	}
}

// CanEdit возвращает true, если роль допускает изменения проекта.
func (r ProjectRole) CanEdit() bool {
	return r == RoleOwner || r == RoleEditor
}

// CanManage возвращает true, если роль допускает управление членами.
func (r ProjectRole) CanManage() bool {
	return r == RoleOwner
}
