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
//   - RAIL-P: Len 4195 == X-проекция, развёртка по объёму ≈5160 (S-206);
//   - RIGL-001: Len 1224,2 == X-проекция, развёртка ≈1506–1518 (S-206);
//   - фасонные покупные: в манифесте только замеры, геометрию строит
//     BuildRailingDecor;
//   - COS-001: пилообразный профиль вычисляется (пилу нельзя
//     масштабировать по числу ступеней — решение владельца).
var registry = map[string]templateSpec{
	"COS-001":  {role: "stringer", kind: TemplateComputed, reason: "пилообразный профиль вычисляется движком"},
	"WEB-001":  {role: "web", kind: TemplateComputed, reason: "отверстия не входят в манифест"},
	"HEEL-001": {role: "heel", kind: TemplateComputed, reason: "отверстия не входят в манифест"},
	"ANCR-001": {role: "anchor", kind: TemplateComputed, reason: "отверстия не входят в манифест"},
	"FWEB-001": {role: "first_web", kind: TemplateComputed, reason: "отверстия не входят в манифест"},
	"TANC-001": {role: "anchor", kind: TemplateComputed, reason: "отверстия не входят в манифест"},
	"TRED-001": {role: "tread", kind: TemplateExact},
	"RISR-001": {role: "riser", kind: TemplateExact},
	"RAIL-P":   {role: "rail_top", kind: TemplateComputed, reason: "Len в каталоге — проекция, не развёртка (S-206)"},
	"POST-001": {role: "post", kind: TemplateExact},
	"RIGL-001": {role: "rigel", kind: TemplateComputed, reason: "Len в каталоге — проекция, не развёртка (S-206)"},
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
			s := tubeAlong(q, kerngeo.Point3{}, kerngeo.Point3{X: 0, Y: 0, Z: length},
				od, wall, role)
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
			// Проступь/подступенок эталона — почти коробки (проступь
			// несёт обработку кромок −0,2 % объёма). Коробка — первое
			// приближение; кромки — работа S-204+, а не этого шаблона.
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
