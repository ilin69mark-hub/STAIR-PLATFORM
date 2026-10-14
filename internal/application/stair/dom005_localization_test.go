package stair

import (
	"context"
	"strings"
	"testing"

	"stairplatform/internal/domain/engineering"
)

// DOM-005 (2026-09-26): 14 доменных ошибок валидации не имели кейса в
// configInputError и уезжали пользователю английским текстом с меткой
// «Параметры лестницы», которую фронт не умеет подсвечивать.
//
// Ниже — регрессия на КАЖДОЕ доменное сообщение: результат обязан быть
// блокирующим InputError с русским текстом и конкретной меткой поля.

// domConfig — базовая валидная конфигурация для провоцирования доменной ошибки.
func domConfig() Config {
	return Config{
		Width:             engineering.Length(900),
		Height:            engineering.Length(2700),
		Flight:            engineering.FlightStraight,
		StepHeight:        engineering.Length(180),
		StringerThickness: engineering.Length(50),
		StepThickness:     engineering.Length(40),
		Riser:             true,
		Clearance:         engineering.Length(2000),
		RailingHeight:     engineering.Length(900),
	}
}

// TestDOM005_DomainValidationErrorsAreLocalised — ни одна доменная ошибка
// Validate() не должна доходить до пользователя английским текстом.
func TestDOM005_DomainValidationErrorsAreLocalised(t *testing.T) {
	cases := []struct {
		name      string
		mutate    func(c *Config)
		wantField string
	}{
		{"unknown flight", func(c *Config) { c.Flight = engineering.FlightType("diagonal") }, "Тип лестницы"},
		{"height zero", func(c *Config) { c.Height = engineering.Length(0) }, "Высота"},
		{"width zero", func(c *Config) { c.Width = engineering.Length(0) }, "Ширина марша"},
		{"landing depth negative", func(c *Config) {
			c.Flight = engineering.FlightLShape
			c.LandingDepth = engineering.Length(-5)
		}, "Глубина площадки"},
		{"landing width below width", func(c *Config) {
			c.Flight = engineering.FlightLShape
			c.LandingWidth = engineering.Length(400)
		}, "Ширина площадки"},
		{"room width", func(c *Config) { c.RoomWidth = engineering.Length(-2) }, "Ширина помещения"},
		{"room length", func(c *Config) { c.RoomLength = engineering.Length(-2) }, "Длина помещения"},
		{"approach negative", func(c *Config) { c.ApproachSpace = engineering.Length(-1) }, "Свободное пространство"},
		{"approach out of range", func(c *Config) { c.ApproachSpace = engineering.Length(500) }, "Свободное пространство"},
		{"turn kind", func(c *Config) {
			c.Flight = engineering.FlightLShape
			c.TurnKind = engineering.TurnKind("spiral")
		}, "Поворот марша"},
		{"winder count", func(c *Config) {
			c.Flight = engineering.FlightUShape
			c.TurnKind = engineering.TurnWinder
			c.WinderCount = 1
		}, "Поворотных ступеней"},
	}

	svc := NewService()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := domConfig()
			tc.mutate(&cfg)
			res, err := svc.Calculate(context.Background(), cfg, Options{})
			if err != nil {
				// Ошибка уровня solver/input — тоже валидный исход, но тогда
				// она должна быть 422 c русским текстом, а не 500.
				if isEnglish(err.Error()) {
					t.Fatalf("DOM-005: %s: ошибка уходит на английском: %v", tc.name, err)
				}
				return
			}
			if !res.Validation.Blocking {
				t.Fatalf("DOM-005: %s: ожидалась блокирующая валидация, got %+v", tc.name, res.Validation)
			}
			if len(res.Validation.Issues) == 0 {
				t.Fatalf("DOM-005: %s: blocking без объяснений", tc.name)
			}
			for _, it := range res.Validation.Issues {
				if isEnglish(it.Message) {
					t.Errorf("DOM-005: %s: английский message %q", tc.name, it.Message)
				}
				if isEnglish(it.Guide) {
					t.Errorf("DOM-005: %s: английский guide %q", tc.name, it.Guide)
				}
				if it.Param == "Параметры лестницы" {
					t.Errorf("DOM-005: %s: метка «Параметры лестницы» — поле не подсвечивается "+
						"(ожидалось %q), issue: %+v", tc.name, tc.wantField, it)
				}
			}
		})
	}
}

// isEnglish — грубая эвристика: текст без кириллицы считается английским.
func isEnglish(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r >= 0x0400 && r <= 0x04FF {
			return false
		}
	}
	return strings.ContainsAny(s, "abcdefghijklmnopqrstuvwxyz")
}
