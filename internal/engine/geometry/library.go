package geometry

// library.go — библиотека параметрических шаблонов деталей (S-202).
//
// Шаблон = «размеры-входы → тело-выход». Точка опоры этапа 2: калькулятор
// берёт шаблон из библиотеки и размещает его (Placement уже есть с фазы 0),
// вместо того чтобы строить каждую деталь из параметров проекта.
//
// Exact против Computed — это НЕ «совпадает с эталоном», а «полнота
// параметров»: Exact строится из размеров каталога один в один, без
// дополнительных входов. Computed строится движком, и в Reason записано,
// каких данных не хватает (отверстий нет в манифесте, профиль пилы
// вычисляется, длина в каталоге — проекция, а не развёртка).
// Молчаливых дыр нет: каждая позиция каталога либо Exact, либо Computed
// с причиной — проверяет TestLibraryCoversCatalog.
//
// Роль — строкой со значением dommfg.PartKind. Импортировать domain-слой
// ради констант сюда нельзя: у geometry нет ребра в manufacturing,
// и новое ребро ради строковой константы — плохая сделка.

import (
	"fmt"

	"stairplatform/internal/domain/engineering"
	kerngeo "stairplatform/internal/geometry"
)

// TubeAt — труба OD×wall длиной length от точки base вдоль +Z (S-204).
//
// Тот же tubeAlong, что строит движок: стойки ограждения идут через него,
// а не мимо. Одно определение трубы на движок и библиотеку — иначе две
// правды о трубе, и расхождение проявится в готовом изделии.
func TubeAt(q Quality, base kerngeo.Point3, od, wall, length float64, role string) *kerngeo.Solid {
	return tubeAlong(q, base, base.Add(kerngeo.NewVector3(0, 0, length)), od, wall, role)
}

// NosedSlab — плита со скруглённым носом (проступь, S-204): сечение
// plateSection + вытяжка вдоль Y. Параметры — те же, что считает builder
// (x0, x1, низ, толщина, радиус, сторона, y0, y1): одно определение
// проступи на движок и библиотеку.
func NosedSlab(q Quality, x0, x1, zBottom, th, noseR float64, side roundSide, y0, y1 float64, role string) (*kerngeo.Solid, error) {
	section, err := plateSection(q, x0, x1, zBottom, th, noseR, side)
	if err != nil {
		return nil, err
	}
	solid, err := kerngeo.Extrude(shiftY(section, y0), kerngeo.NewVector3(0, 1, 0), y1-y0)
	if err != nil {
		return nil, err
	}
	return solid.WithRole(role), nil
}

// SlabX — вертикальное полотно толщиной вдоль X (подступенок, S-204):
// прямоугольник в плоскости YZ + вытяжка. Одно определение подступенка.
func SlabX(x0, y0, y1, z0, z1, th float64, role string) (*kerngeo.Solid, error) {
	profile := []kerngeo.Point3{
		kerngeo.NewPoint3(x0, y0, z0),
		kerngeo.NewPoint3(x0, y0, z1),
		kerngeo.NewPoint3(x0, y1, z1),
		kerngeo.NewPoint3(x0, y1, z0),
	}
	solid, err := kerngeo.Extrude(profile, kerngeo.NewVector3(1, 0, 0), th)
	if err != nil {
		return nil, err
	}
	return solid.WithRole(role), nil
}

// TemplateKind — точный шаблон или вычисляемая деталь.
type TemplateKind string

const (
	// TemplateExact — тело строится из размеров каталога один в один.
	TemplateExact TemplateKind = "exact"
	// TemplateComputed — тело строит движок; Reason объясняет, чего
	// не хватает для шаблона.
	TemplateComputed TemplateKind = "computed"
)

// Template — запись библиотеки на одну позицию каталога.
type Template struct {
	// PartID — ID позиции (COS-001…ENDP-001).
	PartID string
	// Role — роль тела, значение dommfg.PartKind.
	Role string
	// Kind — exact или computed.
	Kind TemplateKind
	// Reason — только для computed: чего не хватает для шаблона.
	Reason string
	// Build — строит тело из размеров каталога. Система координат
	// каноническая: коробка — от начала координат вдоль +X/+Y/+Z,
	// труба — отрезок [0, Len] на оси Z. Размещение — делом вызывающего
	// через Placement (S-204); шаблон места не знает.
	Build func(q Quality) (*kerngeo.Solid, error)
}

// templateSpec — строка реестра: позиция каталога + её статус.
type templateSpec struct {
	role   string
	kind   TemplateKind
	reason string
}

// registry — статус каждой позиции каталога.
//
// Причины computed — замеренные, а не предположенные:
//   - пластины с отверстиями: отверстия не входят в манифест, пластина
//     без них неверна на 3–10 % объёма (HEEL −2,8 %, WEB −3,0 %,
//     ANCR −6,4 %, FWEB −5,0 %, TANC −9,6 %);
//   - фасонные покупные: в манифесте только замеры, геометрию строит
//     BuildRailingDecor;
//   - COS-001: пилообразный профиль вычисляется (пилу нельзя
//     масштабировать по числу ступеней — решение владельца).
//
// Наклонные трубы (RAIL-P, RIGL-001) были computed из-за S-206
// (Len — проекция, не развёртка); после исправления каталога — exact.
var registry = map[string]templateSpec{
	"COS-001":  {role: "stringer", kind: TemplateComputed, reason: "пилообразный профиль вычисляется движком"},
	"WEB-001":  {role: "web", kind: TemplateComputed, reason: "отверстия не входят в манифест"},
	"HEEL-001": {role: "heel", kind: TemplateComputed, reason: "отверстия не входят в манифест"},
	"ANCR-001": {role: "anchor", kind: TemplateComputed, reason: "отверстия не входят в манифест"},
	"FWEB-001": {role: "first_web", kind: TemplateComputed, reason: "отверстия не входят в манифест"},
	"TANC-001": {role: "anchor", kind: TemplateComputed, reason: "отверстия не входят в манифест"},
	"TRED-001": {role: "tread", kind: TemplateExact},
	"RISR-001": {role: "riser", kind: TemplateExact},
	"RAIL-P":   {role: "rail_top", kind: TemplateExact},
	"POST-001": {role: "post", kind: TemplateExact},
	"RIGL-001": {role: "rigel", kind: TemplateExact},
	"TIP-001":  {role: "tip", kind: TemplateComputed, reason: "фасонный покупной, строит BuildRailingDecor"},
	"FLNG-001": {role: "flange", kind: TemplateComputed, reason: "фасонный покупной, строит BuildRailingDecor"},
	"SLIP-001": {role: "slip", kind: TemplateComputed, reason: "фасонный покупной, строит BuildRailingDecor"},
	"DECO-001": {role: "deco", kind: TemplateComputed, reason: "фасонный покупной, строит BuildRailingDecor"},
	"ENDP-001": {role: "end_cap", kind: TemplateComputed, reason: "фасонный покупной, строит BuildRailingDecor"},
}

// Library собирает шаблоны всех позиций каталога. Неизвестная позиция
// в каталоге без записи в реестре — ошибка, а не пропуск: иначе новая
// деталь появится в производстве раньше, чем в библиотеке.
func Library() ([]Template, error) {
	all := engineering.AllParts()
	out := make([]Template, 0, len(all))
	for _, spec := range all {
		reg, ok := registry[spec.ID]
		if !ok {
			return nil, fmt.Errorf("library: позиция %s (%s) без записи в реестре шаблонов",
				spec.ID, spec.Name)
		}
		t := Template{PartID: spec.ID, Role: reg.role, Kind: reg.kind, Reason: reg.reason}
		if reg.kind == TemplateExact {
			build, err := exactBuilder(spec, reg.role)
			if err != nil {
				return nil, fmt.Errorf("library: %s: %w", spec.ID, err)
			}
			t.Build = build
		}
		out = append(out, t)
	}
	return out, nil
}

// exactBuilder — строитель точного шаблона из размеров каталога.
// Точных форм две: труба (FormTube) и сплошная коробка (FormSheet без
// отверстий — проступь и подступенок). Остальные формы сюда попадать
// не должны: это означало бы, что реестр врёт про exact.
func exactBuilder(spec engineering.PartSpec, role string) (func(Quality) (*kerngeo.Solid, error), error) {
	switch spec.Form {
	case engineering.FormTube:
		od, wall, length := spec.TubeOD, spec.TubeWall, spec.Len
		if od <= 2*wall || length <= 0 {
			return nil, fmt.Errorf("труба с OD=%v wall=%v len=%v невозможна", od, wall, length)
		}
		return func(q Quality) (*kerngeo.Solid, error) {
			s := TubeAt(q, kerngeo.Point3{}, od, wall, length, role)
			if s == nil {
				return nil, fmt.Errorf("tubeAlong не построил трубу %v/%v/%v", od, wall, length)
			}
			return s, nil
		}, nil
	case engineering.FormSheet:
		l, w, th := spec.SheetL, spec.SheetW, spec.SheetT
		if l <= 0 || w <= 0 || th <= 0 {
			return nil, fmt.Errorf("коробка с L=%v W=%v T=%v невозможна", l, w, th)
		}
		return func(Quality) (*kerngeo.Solid, error) {
			// Коробка каталога — габариты заготовки БЕЗ чистовой обработки:
			// филёнка R10 и подобные — дело проектного конструктора
			// NosedSlab, а не каталога. Разделять их — значит не путать
			// размер заготовки с размером готового изделия.
			prof := []kerngeo.Point3{
				{X: 0, Y: 0, Z: 0}, {X: l, Y: 0, Z: 0},
				{X: l, Y: w, Z: 0}, {X: 0, Y: w, Z: 0},
			}
			s, err := kerngeo.Extrude(prof, kerngeo.NewVector3(0, 0, 1), th)
			if err != nil {
				return nil, fmt.Errorf("extrude коробки %v/%v/%v: %w", l, w, th, err)
			}
			return s, nil
		}, nil
	case engineering.FormPart:
		// Фасонная покупная деталь (наконечник, фланец, заглушка): в каталоге
		// только замеры, геометрию строит движок. Явная ветка, а не default:
		// линтер exhaustive требует видеть все формы.
		return nil, fmt.Errorf("форма %q не бывает точным шаблоном", spec.Form)
	default:
		return nil, fmt.Errorf("форма %q не бывает точным шаблоном", spec.Form)
	}
}
