// Package manufacturing реализует EDM для Manufacturing Platform
// (BC-007, MFG-0001): преобразование валидной инженерной модели в
// комплект производственных данных — детали, материалы, BOM и карту
// раскроя. Manufacturing не изменяет конструкцию, а лишь определяет,
// как её изготовить.
// Единицы: мм/кг по ADR-0008; размеры используют engineering.Length.
// Домен не зависит от HTTP, БД, ORM, UI и AI (ADR-0006).
package manufacturing

import (
	"fmt"
	"math"

	"stairplatform/internal/domain/engineering"
)

// MaterialCode — уникальный код материала в каталоге (MFG-0005).
type MaterialCode string

// Material — неизменяемая запись каталога материалов (MFG-0005).
// Материал является обязательным атрибутом каждой производственной
// детали. Для MVP хранятся свойства, используемые декомпозицией,
// BOM и cut list; прочностные и стоимостные свойства добавляются
// на следующих этапах.
type Material struct {
	Code MaterialCode
	// Name — техническое имя каталога (английский, используется в
	// производственных документах и логах).
	Name string
	// NameRu — витринное имя на русском. Пустое значение допустимо: тогда
	// клиент показывает Name (витрина не ломается на нелокализованном коде).
	NameRu       string
	Category     string
	Density      float64 // кг/м³
	MinThickness float64 // мм
	MaxThickness float64 // мм
	// MaxWidthMm — максимальная ширина марша, гарантируемая изготовлением
	// именно в этом материале (MFG-0012: энвелоп раскройных листов).
	MaxWidthMm float64
	// MaxHeightMm — максимальная высота подъёма, гарантируемая изготовлением
	// именно в этом материале (MFG-0012: крупнейший лист для косоура).
	MaxHeightMm float64
}

// Validate проверяет корректность записи материала.
func (m *Material) Validate() error {
	if m == nil {
		return fmt.Errorf("manufacturing: material is required")
	}
	if m.Code == "" {
		return fmt.Errorf("manufacturing: material code must not be empty")
	}
	if m.Name == "" {
		return fmt.Errorf("manufacturing: material name must not be empty")
	}
	if m.Category == "" {
		return fmt.Errorf("manufacturing: material category must not be empty")
	}
	if math.IsNaN(m.Density) || math.IsInf(m.Density, 0) || m.Density <= 0 {
		return fmt.Errorf("manufacturing: material density must be positive, got %v", m.Density)
	}
	if math.IsNaN(m.MinThickness) || math.IsNaN(m.MaxThickness) ||
		m.MinThickness < 0 || m.MaxThickness < m.MinThickness {
		return fmt.Errorf("manufacturing: material thickness range is invalid")
	}
	if math.IsNaN(m.MaxWidthMm) || m.MaxWidthMm <= 0 {
		return fmt.Errorf("manufacturing: material max width must be positive")
	}
	if math.IsNaN(m.MaxHeightMm) || m.MaxHeightMm <= 0 {
		return fmt.Errorf("manufacturing: material max height must be positive")
	}
	return nil
}

// SupportsThickness проверяет, что материал выпускается в толщине t (мм).
func (m *Material) SupportsThickness(t float64) bool {
	if m == nil {
		return false
	}
	return t >= m.MinThickness-engineering.Precision && t <= m.MaxThickness+engineering.Precision
}

// MaterialRegistry — детерминированный in-memory каталог материалов
// (MFG-0005). Реестр неизменяем после создания: одна ревизия — один
// каталог; порядок материалов фиксирован.
type MaterialRegistry struct {
	byCode map[MaterialCode]*Material
	order  []MaterialCode
}

// NewMaterialRegistry создаёт реестр; дубликаты кодов и невалидные
// записи отклоняются.
func NewMaterialRegistry(materials ...*Material) (*MaterialRegistry, error) {
	r := &MaterialRegistry{
		byCode: make(map[MaterialCode]*Material, len(materials)),
		order:  make([]MaterialCode, 0, len(materials)),
	}
	for _, m := range materials {
		if err := m.Validate(); err != nil {
			return nil, err
		}
		if _, exists := r.byCode[m.Code]; exists {
			return nil, fmt.Errorf("manufacturing: duplicate material code %q", m.Code)
		}
		r.byCode[m.Code] = m
		r.order = append(r.order, m.Code)
	}
	return r, nil
}

// Find возвращает материал по коду.
func (r *MaterialRegistry) Find(code MaterialCode) (*Material, bool) {
	if r == nil {
		return nil, false
	}
	m, ok := r.byCode[code]
	return m, ok
}

// Materials возвращает материалы в порядке добавления (детерминизм).
func (r *MaterialRegistry) Materials() []*Material {
	if r == nil {
		return nil
	}
	out := make([]*Material, 0, len(r.order))
	for _, code := range r.order {
		out = append(out, r.byCode[code])
	}
	return out
}
