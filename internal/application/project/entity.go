// Package project реализует application layer проектов (BC-001):
// создание проекта, сохранение конфигурации лестницы и расчёта
// (BE-0002 Use Cases, Workflow Orchestration). Application зависит от
// порта Repository (BE-0005) и оркестрирует доменные сервисы и движки;
// не зависит от транспорта и БД.
package project

import (
	"time"
)

// Project — корневая сущность платформы (BC-001).
type Project struct {
	ID          string
	Name        string
	Description string
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// StairConfiguration — сохранённая конфигурация лестницы проекта
// (BC-002). Параметры в мм; ComfortStep в мм.
type StairConfiguration struct {
	ID                  string
	ProjectID           string
	WidthMM             float64
	HeightMM            float64
	Flight              string
	StepHeightMM        float64
	StringerThicknessMM float64
	StepThicknessMM     float64
	ClearanceMM         float64
	RailingHeightMM     float64
	ComfortStepMM       float64
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// Calculation — сохранённый результат конвейера (снапшот). Result —
// JSON-сериализованный полный результат (stair.Result); отдельные
// представления дублируют части для выборочных запросов.
type Calculation struct {
	ID              string
	ProjectID       string
	ConfigurationID string
	Valid           bool
	Blocking        bool
	Result          []byte // JSON: полный результат
	CreatedAt       time.Time
}
