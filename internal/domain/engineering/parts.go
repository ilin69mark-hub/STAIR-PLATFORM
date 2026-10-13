// parts.go — КАТАЛОГ ДЕТАЛЕЙ ЛЕСТНИЦЫ.
//
// Это единственный источник правды о деталях. До появления этого файла
// размеры деталей жили в трёх местах одновременно (FreeCAD-скрипт,
// константы в railing.go, тесты), из-за чего приходилось сверять их
// вручную — источник бесконечных правок геометрии.
//
// Каталог питает три потребителя:
//   - геометрию (BuildRailingDecor строит детали из этих записей),
//   - раскрой (GroupCutting — плоские детали, которые режутся из листа),
//   - спецификацию и прайс (GroupPurchased — покупные детали ограждения).
//
// Крепёжа в каталоге НЕТ. Метизы — поштучные позиции другого справочника,
// dommfg.DefaultFastenerRegistry() (internal/domain/manufacturing/fastener.go):
// у него есть код, размер и вид, цены строятся перебором каталога, и он один
// на движок, магазин и прайс. Здесь крепёж был продублирован двумя строками
// (ANCH-010, SCR-005) без размеров и без единого потребителя, и эти строки
// молча разошлись с настоящим каталогом: «Анкер Ø10» не совпадал ни с одним
// кодом вида SCREW-*/PLUG-*. Две правды о крепеже, из которых работала одна.
//
// Размеры сняты из эталона freecad-out/stair-comb-railings.FCStd скриптом
// parts_measure.py. Эталон — источник истины; этот файл — его перенос.
package engineering

// PartGroup — группа детали. Определяет, что с деталью делает производство.
type PartGroup string

const (
	// GroupCutting — деталь режется из листа у нас. Для неё нужны точные
	// плоские контуры: они попадают в карту раскроя.
	GroupCutting PartGroup = "cutting"

	// GroupPurchased — готовая покупная единица. Режется только по длине
	// (если это труба) либо приходит в заводской упаковке. В раскрой листа
	// не идёт, точные контуры не нужны — достаточно корректного 3D-вида.
	GroupPurchased PartGroup = "purchased"
)

// PartMaterial — материал детали.
type PartMaterial string

const (
	MatSteel     PartMaterial = "steel"     // сталь (косоур, приварные пластины)
	MatWood      PartMaterial = "wood"      // дерево (проступь, подступенок)
	MatStainless PartMaterial = "stainless" // нержавейка AISI 304 (ограждение)
	MatPlastic   PartMaterial = "plastic"   // заглушки
)

// PartForm — тип заготовки. Определяет, как деталь попадает в производство.
type PartForm string

const (
	// FormSheet — плоская деталь из листа (раскрой).
	FormSheet PartForm = "sheet"

	// FormTube — труба, режется по длине.
	FormTube PartForm = "tube"

	// FormPart — покупной узел, приходит готовым (наконечник регулируемый,
	// фланец + накидная часть + накладка как комплект).
	FormPart PartForm = "part"
)

// PartSpec — запись каталога деталей.
type PartSpec struct {
	// ID — устойчивый идентификатор для чертежей и спецификации.
	ID string

	// Name — человекочитаемое имя.
	Name string

	// Group — группа детали.
	Group PartGroup

	// Form — тип заготовки.
	Form PartForm

	// Material — материал.
	Material PartMaterial

	// Note — пояснение для спецификации (как в чертеже).
	Note string

	// TubeOD, TubeWall — для труб (FormTube): наружный диаметр и толщина
	// стенки, мм. Ноль для не-труб.
	TubeOD, TubeWall float64

	// Len — длина трубы ПО ОСИ, мм (FormTube): то, что режут в цехе.
	// Горизонтальная проекция наклонной трубы здесь запрещена: это
	// −20 % длины, и мастер отрежет короткую трубу (S-206). Для
	// вертикальных труб ось совпадает с высотой — расхождения нет.
	Len float64

	// CutLen — длина реза трубы с учётом припуска/подрезки, мм.
	// Для труб это то, что идёт в производство; Len — чистая геометрия.
	// Решение владельца: припуска нет, режем в размер (S-206),
	// поэтому CutLen == Len — честно, а не «забыли заполнить».
	CutLen float64

	// SheetL, SheetW, SheetT — габарит плоской детали из листа, мм
	// (FormSheet). Объём листа для раскроя считается по этим размерам.
	SheetL, SheetW, SheetT float64

	// DimL, DimW, DimH — габарит покупной детали-фитинга, мм (FormPart):
	// наконечник, фланец, муфта, накладка, заглушка. Для труб не заполняется
	// (там своя геометрия), для листа тоже (там Sheet*).
	//
	// Зачем это поле (S-180). Раньше эти размеры жили только в константах
	// manufacturing/railing_parts.go, а каталог знал лишь названия и
	// материалы. Итог: две копии одного и того же, и расхождение между ними
	// проявилось бы в готовом изделии, а не в тесте. Теперь фитинги описаны
	// здесь, а производство читает размеры отсюда.
	DimL, DimW, DimH float64

	// MassPerUnit — масса детали, кг (заполняется по материалу и объёму).
	MassPerUnit float64
}

// CuttingParts — детали, которые идут в раскрой листа.
func CuttingParts() []PartSpec {
	return []PartSpec{
		// --- Косоур и приварные пластины: сталь ---
		{
			ID: "COS-001", Name: "Косоур", Group: GroupCutting, Form: FormSheet,
			Material: MatSteel,
			Note:     "Боковая несущая балка прямого марша",
			SheetL:   3840, SheetW: 2760, SheetT: 8,
		},
		{
			ID: "WEB-001", Name: "Перемычка", Group: GroupCutting, Form: FormSheet,
			Material: MatSteel,
			Note:     "Приваривается к косоуру — облегчение/связь",
			SheetL:   50, SheetW: 161, SheetT: 4,
		},
		{
			ID: "HEEL-001", Name: "Пятка", Group: GroupCutting, Form: FormSheet,
			Material: MatSteel,
			Note:     "Приваривается к косоуру — нижний узел",
			SheetL:   230, SheetW: 50, SheetT: 4,
		},
		{
			ID: "ANCR-001", Name: "Анкер-пластина", Group: GroupCutting, Form: FormSheet,
			Material: MatSteel,
			Note:     "Приваривается к косоуру — крепление к полу",
			SheetL:   240, SheetW: 50, SheetT: 4,
		},
		{
			ID: "FWEB-001", Name: "Первая перемычка", Group: GroupCutting, Form: FormSheet,
			Material: MatSteel,
			Note:     "Приваривается к косоуру — узел первой ступени",
			SheetL:   40, SheetW: 117, SheetT: 4,
		},
		{
			ID: "TANC-001", Name: "Верхняя распорка", Group: GroupCutting, Form: FormSheet,
			Material: MatSteel,
			Note:     "Приваривается к косоуру — верхний узел",
			SheetL:   50, SheetW: 161, SheetT: 4,
		},
		// --- Ступени: дерево в текущей конфигурации.
		//     В будущем обе детали переходят на сталь — переключается
		//     материалом, не правкой геометрии (см. StairConfiguration).
		{
			ID: "TRED-001", Name: "Проступь", Group: GroupCutting, Form: FormSheet,
			Material: MatWood,
			Note:     "Проступь прямой лестницы (дуб). В будущей конфигурации — сталь",
			SheetL:   260, SheetW: 900, SheetT: 40,
		},
		{
			ID: "RISR-001", Name: "Подступенок", Group: GroupCutting, Form: FormSheet,
			Material: MatWood,
			Note:     "Подступенок (дуб). В будущей конфигурации — сталь",
			SheetL:   900, SheetW: 135, SheetT: 40,
		},
	}
}

// PurchasedParts — покупные позиции: приходят готовыми, в раскрой не идут.
func PurchasedParts() []PartSpec {
	return []PartSpec{
		{
			ID: "RAIL-P", Name: "Перила (поручень)", Group: GroupPurchased, Form: FormTube,
			Material: MatStainless,
			Note:     "Труба нерж. AISI 304, рез по длине",
			TubeOD:   42.4, TubeWall: 1.5, Len: 5160.8, CutLen: 5160.8,
		},
		{
			ID: "POST-001", Name: "Стойка", Group: GroupPurchased, Form: FormTube,
			Material: MatStainless,
			Note:     "Труба нерж. AISI 304, рез по длине",
			TubeOD:   42.4, TubeWall: 1.5, Len: 805.7, CutLen: 805.7,
		},
		{
			ID: "RIGL-001", Name: "Ригель", Group: GroupPurchased, Form: FormTube,
			Material: MatStainless,
			Note:     "Трубка нерж. AISI 304, два ряда, рез по длине",
			TubeOD:   12.0, TubeWall: 1.0, Len: 1506.3, CutLen: 1506.3,
		},
		{
			ID: "TIP-001", Name: "Наконечник регулируемый", Group: GroupPurchased, Form: FormPart,
			DimL: 70.0, DimW: 50.8, DimH: 95.4,
			Material: MatStainless,
			Note:     "Покупной регулируемый фитинг под поручень. Несёт поручень, крепится на стойку",
		},
		{
			ID: "FLNG-001", Name: "Фланец опоры", Group: GroupPurchased, Form: FormPart,
			DimL: 86.0, DimW: 86.0, DimH: 8.0,
			Material: MatStainless,
			Note:     "Крепёжный фланец стойки, 3 отв. Ø9",
		},
		{
			ID: "SLIP-001", Name: "Накидная часть", Group: GroupPurchased, Form: FormPart,
			DimL: 43.0, DimW: 43.0, DimH: 805.7,
			Material: MatStainless,
			Note:     "Муфта-обойма, накрывает стойку",
		},
		{
			ID: "DECO-001", Name: "Декоративная накладка", Group: GroupPurchased, Form: FormPart,
			DimL: 88.0, DimW: 88.0, DimH: 17.0,
			Material: MatStainless,
			Note:     "Штампованный колпачок, скрывает посадку стойки",
		},
		{
			ID: "ENDP-001", Name: "Торцевая заглушка", Group: GroupPurchased, Form: FormPart,
			DimL: 39.2, DimW: 39.2, DimH: 39.2,
			Material: MatPlastic,
			Note:     "Заглушка торца трубы Ø39,2",
		},
	}
}

// AllParts — полный каталог в порядке групп: раскрой → покупное.
func AllParts() []PartSpec {
	all := make([]PartSpec, 0, 16)
	all = append(all, CuttingParts()...)
	all = append(all, PurchasedParts()...)
	return all
}

// PartsByGroup — выборка каталога по группе.
func PartsByGroup(g PartGroup) []PartSpec {
	var out []PartSpec
	for _, p := range AllParts() {
		if p.Group == g {
			out = append(out, p)
		}
	}
	return out
}

// PartByID — поиск детали по идентификатору.
func PartByID(id string) (PartSpec, bool) {
	for _, p := range AllParts() {
		if p.ID == id {
			return p, true
		}
	}
	return PartSpec{}, false
}
