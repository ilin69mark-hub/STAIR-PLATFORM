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

// Permission — именованное право на операцию в проекте (EDR-0015 §3.3).
// Владеет роль участника проекта (ProjectRole); проверка — через
// ProjectRole.HasPermission.
type Permission string

const (
	// PermissionProjectRead — чтение проекта (все члены).
	PermissionProjectRead Permission = "project.read"
	// PermissionProjectEdit — изменение проекта/конфигурации (owner/editor).
	PermissionProjectEdit Permission = "project.edit"
	// PermissionProjectManage — управление членами и решения (owner).
	PermissionProjectManage Permission = "project.manage"
)

// Permissions возвращает набор прав роли (матрица EDR-0015 §3.3).
func (r ProjectRole) Permissions() []Permission {
	switch r {
	case RoleOwner:
		return []Permission{PermissionProjectRead, PermissionProjectEdit, PermissionProjectManage}
	case RoleEditor:
		return []Permission{PermissionProjectRead, PermissionProjectEdit}
	default: // RoleViewer
		return []Permission{PermissionProjectRead}
	}
}

// HasPermission возвращает true, если роль обладает правом p.
func (r ProjectRole) HasPermission(p Permission) bool {
	for _, perm := range r.Permissions() {
		if perm == p {
			return true
		}
	}
	return false
}

// CanEdit возвращает true, если роль допускает изменения проекта.
// Совместимая обёртка над PermissionProjectEdit (EDR-0015 §3.3).
func (r ProjectRole) CanEdit() bool {
	return r.HasPermission(PermissionProjectEdit)
}

// CanManage возвращает true, если роль допускает управление членами.
// Совместимая обёртка над PermissionProjectManage (EDR-0015 §3.3).
func (r ProjectRole) CanManage() bool {
	return r.HasPermission(PermissionProjectManage)
}
