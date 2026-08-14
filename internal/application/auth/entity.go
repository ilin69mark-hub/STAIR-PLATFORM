// Package auth реализует прикладной слой Identity/Authentication
// (SEC-0003) и базовую модель Authorization (SEC-0004): учётные записи,
// роли, серверные сессии с opaque-токенами, tenant-привязку пользователя
// (SEC-0005). Transport зависит от интерфейса Service; БД — от порта
// Repository (BE-0005, инверсия зависимостей DOM-0008).
package auth

import (
	"context"
	"time"
)

// Role — роль пользователя (SEC-0004).
type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

// Status — состояние учётной записи (SEC-0003).
type Status string

const (
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
)

// Tenant — граница изоляции данных (SEC-0005).
type Tenant struct {
	ID        string
	Name      string
	Slug      string
	CreatedAt time.Time
}

// User — учётная запись (SEC-0003).
type User struct {
	ID           string
	TenantID     string
	Email        string
	Name         string
	PasswordHash string
	Role         Role
	Status       Status
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Session — серверная сессия. TokenHash — SHA-256 от opaque-токена;
// открытый токен клиент держит только в cookie (httpOnly).
type Session struct {
	ID        string
	UserID    string
	TokenHash string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Repository — порт доступа к данным auth (BE-0005).
type Repository interface {
	// DefaultTenant возвращает дефолтный tenant (slug "default").
	DefaultTenant(ctx context.Context) (*Tenant, error)

	// CreateUser создаёт пользователя; ErrEmailExists — email занят.
	CreateUser(ctx context.Context, u *User) error
	// GetUserByEmail возвращает пользователя по email; ErrNotFound — нет.
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	// GetUserByID возвращает пользователя по ID; ErrNotFound — нет.
	GetUserByID(ctx context.Context, id string) (*User, error)

	// CreateSession сохраняет сессию (токен уже захэширован).
	CreateSession(ctx context.Context, s *Session) error
	// GetSessionByTokenHash возвращает сессию по хешу токена;
	// ErrNotFound — сессии нет.
	GetSessionByTokenHash(ctx context.Context, tokenHash string) (*Session, error)
	// DeleteSessionByTokenHash удаляет сессию (лог-аут).
	DeleteSessionByTokenHash(ctx context.Context, tokenHash string) error
}
