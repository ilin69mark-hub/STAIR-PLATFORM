// Package models — эталон FreeCAD, вшитый в бинарь (S-199).
//
// models/parts-manifest.json генерируется скриптом
// hack/gen_part_manifest.py из эталонной модели
// models/reference/stair-comb-railings.FCStd и описания
// models/parts.yaml. Здесь он вшит через go:embed, чтобы гейты
// сверялись с манифестом из репозитория, а не искали эталон
// на диске (см. S-194: поиск вверх по дереву со Skip — запрещён).
//
// Правило: этот пакет — только данные. Никакой логики сверки здесь нет,
// она живёт в тестах потребителя (internal/domain/engineering).
package models

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed parts-manifest.json
var partsManifestJSON []byte

// Box — габарит: длина × ширина × высота, мм.
type Box struct {
	L float64 `json:"l"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// SubSolid — тело внутри объекта FreeCAD. Нужно, потому что «одна деталь»
// не всегда «одно тело»: наконечник TIP-001 — несколько тел на стойку.
// Без этого списка манифест врал бы про состав детали.
type SubSolid struct {
	BBox   Box     `json:"bbox"`
	Volume float64 `json:"volume"`
}

// Solid — объект FreeCAD, относящийся к детали.
type Solid struct {
	Object    string     `json:"object"`
	BBox      Box        `json:"bbox"`
	Volume    float64    `json:"volume"`
	SubSolids []SubSolid `json:"subsolids"`
}

// Tube — диаметры трубы, которые нельзя снять с bbox (фланец сплавлен
// со стойкой в эталоне; источник — константы fix_railings.py).
type Tube struct {
	OD   float64 `json:"od"`
	Wall float64 `json:"wall"`
}

// Part — деталь манифеста: смысл из parts.yaml + замеры из модели.
type Part struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Role         string  `json:"role"`
	Group        string  `json:"group"`
	Form         string  `json:"form"`
	Material     string  `json:"material"`
	SourceObject string  `json:"source_object"`
	Count        int     `json:"count"`
	BBox         Box     `json:"bbox"`
	Volume       float64 `json:"volume"`
	Tube         *Tube   `json:"tube,omitempty"`
	Solids       []Solid `json:"solids"`
	Note         string  `json:"note,omitempty"`
}

// Manifest — корень parts-manifest.json.
type Manifest struct {
	GeneratedBy       string   `json:"generated_by"`
	FreeCADVersion    []string `json:"freecad_version"`
	SourceModel       string   `json:"source_model"`
	SourceSHA256      string   `json:"source_sha256"`
	ToleranceMM       float64  `json:"tolerance_mm"`
	ToleranceVolumePC float64  `json:"tolerance_volume_pct"`
	Parts             []Part   `json:"parts"`
}

// Load разбирает вшитый манифест. Ошибка здесь — это не «битый JSON»,
// а «манифест в репозитории испорчен»: падать должно громко, в тесте,
// а не тихим нулем в проде (поэтому нет Must-варианта с panic).
func Load() (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(partsManifestJSON, &m); err != nil {
		return Manifest{}, fmt.Errorf("models: разбор parts-manifest.json: %w", err)
	}
	if len(m.Parts) == 0 {
		return Manifest{}, fmt.Errorf("models: parts-manifest.json без деталей")
	}
	return m, nil
}

// ByID ищет деталь по идентификатору каталога.
func (m Manifest) ByID(id string) (Part, bool) {
	for _, p := range m.Parts {
		if p.ID == id {
			return p, true
		}
	}
	return Part{}, false
}
