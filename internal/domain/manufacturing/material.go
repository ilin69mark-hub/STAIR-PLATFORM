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
	// FixedTreadThicknessMm — толщина ступени, НАСТРАИВАТЬ КОТОРУЮ НЕЛЬЗЯ.
	// Ноль означает «толщину ступени выбирает пользователь в пределах
	// MinTreadThickness…MaxThickness».
	//
	// Металлическая ступень — деталь из листа проката: покупатель не
	// заказывает «ступень 6 мм», он заказывает готовую ступень изделия. Толщина
	// 5 мм зафиксирована решением владельца 2026-10-05, и выбор в калькуляторе
	// убран: он предлагал диапазон 3–8 мм, которого в изделии нет.
	//
	// Отдельное поле, а не ужесточение MinThickness: у металла 3–12 мм идут
	// косоур, площадка и обвязка, а ступень у него одна толщина.
	FixedTreadThicknessMm float64
	// PurchasedOnly — материал только для ПОКУПНЫХ деталей (ограждение).
	//
	// Такие детали приходят готовыми с завода: они не раскраиваются из листа
	// и не режутся на станке, поэтому у материала НЕТ листов в каталоге
	// MFG-0012 и НЕТ скорости реза в MFG-0009 — и это нормально, а не
	// нарушение каталога.
	//
	// Флаг добавлен вместе с покупными деталями ограждения: инвариант
	// «у каждого материала есть лист и скорость реза» для нержавейки
	// ограждения неприменим, и без флага проверка каталога падала.
	PurchasedOnly bool
	// MinTreadThicknessMm — минимальная толщина ИМЕННО СТУПЕНИ из этого
	// материала, мм. Ноль означает «действует общий MinThickness».
	//
	// Отдельное поле, а не ужесточение MinThickness, потому что ступень и
	// косоур — разные детали: правило 40 мм у дуба относится к проступи под
	// скругление R10, а косоур 20 мм из той же породы остаётся допустимым.
	// Решение владельца 2026-10-05.
	MinTreadThicknessMm float64
	// MaxWidthMm — максимальная ширина марша, гарантируемая изготовлением
	// именно в этом материале (MFG-0012: энвелоп раскройных листов).
	MaxWidthMm float64
	// MaxHeightMm — максимальная высота подъёма, гарантируемая изготовлением
	// именно в этом материале (MFG-0012: крупнейший лист для косоура).
	MaxHeightMm float64
	// TreadNoseRadiusMm — радиус скругления носа ступени, который изготовитель
	// делает по умолчанию в этом материале. Это НЕ украшение, а операция:
	// скругление снимает материал и требует фрезеровки, поэтому входит в
	// цену (см. milling:chamfer в разбивке). У металла ноль: фасок на
	// прокатанном/шлифованном профиле нет, и выставлять их — значит считать
	// покупателю работу, которой не будет.
	//
	// Значение породы подобрано так, чтобы скругление помещалось в толщину
	// ступени (20 мм — минимальная для дерева) и не выходило за пределы
	// нормы: мягкое дерево крошится, поэтому радиус меньше.
	TreadNoseRadiusMm float64
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
	if math.IsNaN(m.TreadNoseRadiusMm) || m.TreadNoseRadiusMm < 0 {
		return fmt.Errorf("manufacturing: material tread nose radius must not be negative")
	}
	// Фиксированная толщина ступени либо не задана (0 — толщину выбирает
	// пользователь), либо лежит внутри общего диапазона толщин: иначе поле
	// противоречит самому себе, и такую ступень нельзя было бы изготовить.
	if math.IsNaN(m.FixedTreadThicknessMm) || m.FixedTreadThicknessMm < 0 ||
		(m.FixedTreadThicknessMm > 0 &&
			(m.FixedTreadThicknessMm < m.MinThickness || m.FixedTreadThicknessMm > m.MaxThickness)) {
		return fmt.Errorf("manufacturing: material %q fixed tread thickness %v is outside thickness range %v–%v",
			m.Code, m.FixedTreadThicknessMm, m.MinThickness, m.MaxThickness)
	}
	// Минимум ступени либо не задан (0 — действует общий MinThickness), либо
	// лежит внутри общего диапазона толщин: иначе поле противоречит самому
	// себе и ступень нельзя было бы изготовить ни при какой толщине.
	if math.IsNaN(m.MinTreadThicknessMm) || m.MinTreadThicknessMm < 0 ||
		(m.MinTreadThicknessMm > 0 &&
			(m.MinTreadThicknessMm < m.MinThickness || m.MinTreadThicknessMm > m.MaxThickness)) {
		return fmt.Errorf("manufacturing: material %q min tread thickness %v is outside thickness range %v–%v",
			m.Code, m.MinTreadThicknessMm, m.MinThickness, m.MaxThickness)
	}
	// Скругление не может быть шире половины минимальной толщины СТУПЕНИ: иначе
	// нос не поместится в деталь. Проверяется по MinTreadThickness, а не по
	// общему MinThickness: с радиусом 10 мм при общем минимуме 20 мм равенство
	// было бы ровно на границе, и порода с чуть меньшим радиусом прошла бы
	// впритык.
	minTread := m.MinThickness
	if m.MinTreadThicknessMm > 0 {
		minTread = m.MinTreadThicknessMm
	}
	if m.TreadNoseRadiusMm > 0 && m.TreadNoseRadiusMm > minTread/2 {
		return fmt.Errorf("manufacturing: material %q nose radius %v exceeds half of min tread thickness %v",
			m.Code, m.TreadNoseRadiusMm, minTread)
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

// HasFixedTreadThickness — задана ли материалу единственная толщина ступени.
func (m *Material) HasFixedTreadThickness() bool {
	return m != nil && m.FixedTreadThicknessMm > 0
}

// MinTreadThickness — действующий минимум толщины ступени: отдельное поле
// MinTreadThicknessMm, а при нуле — общий MinThickness.
func (m *Material) MinTreadThickness() float64 {
	if m == nil || m.MinTreadThicknessMm <= 0 {
		if m == nil {
			return 0
		}
		return m.MinThickness
	}
	return m.MinTreadThicknessMm
}

// SupportsTreadThickness — допускает ли материал ступень такой толщины.
// Верхняя граница общая с деталью, нижняя — MinTreadThickness: у дуба ступень
// от 40 мм, а косоур от 20 мм (решение владельца 2026-10-05).
func (m *Material) SupportsTreadThickness(t float64) bool {
	if m == nil {
		return false
	}
	return t >= m.MinTreadThickness()-engineering.Precision && t <= m.MaxThickness+engineering.Precision
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
