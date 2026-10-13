// railing_parts.go — ПОКУПНЫЕ детали ограждения для спецификации.
//
// Ограждение не раскраивается из листа: стойки, поручень, ригели и фитинги
// приходят готовыми с завода. Раньше они не попадали в производственный пакет
// вообще — геометрия ограждения строится как декорация (BuildRailingDecor),
// а decompose() разбирает только тело модели. В спецификации ограждения не
// было: ни позиций, ни цен.
//
// Здесь детали ограждения собираются из КОНФИГУРАЦИИ, каталога
// engineering.PurchasedParts() и геометрии: диаметры труб и высота стойки
// постоянны (каталог), а длины поручня и ригелей — по оси, из геометрии
// (enggeo.RailingRailAxisLen / RailingRigelAxisLen). Длина по оси, а не
// горизонтальная проекция: смета режет трубу, и проекция короче настоящей
// на ~20 % (S-206).
//
// Длины и диаметры взяты из каталога internal/domain/engineering/parts.go,
// который сверяется с эталоном FreeCAD (TestCatalog*).

package manufacturing

import (
	"fmt"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
	enggeo "stairplatform/internal/engine/geometry"
)

// Каталожные размеры покупных деталей ограждения, мм.
//
// Раньше это были константы со словами «дублируют каталог осознанно», но
// дублирование осознанным быть не может: расхождение копий проявилось бы в
// готовом ограждении, а не в тесте. Теперь единственный источник —
// engineering.PurchasedParts(), а эти переменные читают его (S-180).
//
// Пропавшая позиция каталога — паника на старте, а не нулевая труба в
// готовом изделии: такое место заметили бы на приёмке, когда деньги уже
// потрачены.
var (
	postOD      = purchasedTubeOD("POST-001") // стойка и поручень: полая труба
	postWall    = purchasedTubeWall("POST-001")
	rigelOD     = purchasedTubeOD("RIGL-001") // ригель: полая трубка
	rigelWall   = purchasedTubeWall("RIGL-001")
	balusterLen = purchasedTubeLen("POST-001") // высота стойки под поручень
	tipSizeL    = purchasedDimL("TIP-001")     // наконечник: овал вдоль поручня
	tipSizeW    = purchasedDimW("TIP-001")
	tipSizeH    = purchasedDimH("TIP-001")
	flangeOD    = purchasedDimL("FLNG-001") // фланец опоры
	flangeH     = purchasedDimH("FLNG-001")
	slipOD      = purchasedDimL("SLIP-001") // накидная часть (муфта)
	decoOD      = purchasedDimL("DECO-001") // декоративная накладка
	decoH       = purchasedDimH("DECO-001")
	endCapOD    = purchasedDimL("ENDP-001") // торцевая заглушка
)

// purchasedSpec — позиция каталога по ID. То, что производство строит, но
// каталог не описывает, — ошибка сборки, а не нулевой размер.
func purchasedSpec(id string) engineering.PartSpec {
	for _, p := range engineering.PurchasedParts() {
		if p.ID == id {
			return p
		}
	}
	panic(fmt.Sprintf("покупная деталь %q отсутствует в каталоге engineering.PurchasedParts()", id))
}

func purchasedTubeOD(id string) float64   { return purchasedSpec(id).TubeOD }
func purchasedTubeWall(id string) float64 { return purchasedSpec(id).TubeWall }
func purchasedTubeLen(id string) float64  { return purchasedSpec(id).Len }
func purchasedDimL(id string) float64     { return purchasedSpec(id).DimL }
func purchasedDimW(id string) float64     { return purchasedSpec(id).DimW }
func purchasedDimH(id string) float64     { return purchasedSpec(id).DimH }

// Параметры раскладки ограждения. Это НЕ размеры деталей, а правила
// размещения, поэтому они остались константами: в каталоге им не место.
const (
	// rigelRows — число рядов ригелей (33% и 66% высоты ограждения).
	rigelRows = 2
)

// engLen — короткий перевод float64 (мм) в engineering.Length.
func engLen(mm float64) engineering.Length { return engineering.Length(mm) }

// railingBayLengths — длины ригелей по оси для каждого пролёта между
// соседними стойками. Пролёты не обязаны быть равными (стойки привязаны
// к ближайшим ступеням), поэтому длина у каждого пролёта своя —
// усреднять в один «span» запрещено (S-206).
func railingBayLengths(steps []int, b, h float64) []float64 {
	out := make([]float64, 0, len(steps))
	for i := 0; i+1 < len(steps); i++ {
		out = append(out, enggeo.RailingRigelAxisLen(steps[i], steps[i+1], b, h))
	}
	return out
}

// railingPostIndices — индексы ступеней, на которых стоят стойки.
//
// Делегирует геометрии (enggeo.RailingPostSteps): правило одно, и оно живёт
// в одном месте. Раньше здесь стояла своя копия правила плюс своя формула
// int(run/1500)+1: на марше 15×270 она давала 3 стойки, а геометрия рисовала
// 4 — спецификация и модель расходились (см. историю в git).
func railingPostIndices(n int, b float64) []int {
	return enggeo.RailingPostSteps(n, b)
}

// railingPostCount — сколько стоек на марше.
func railingPostCount(n int, b float64) int {
	return len(railingPostIndices(n, b))
}

// purchasedRailingParts собирает покупные детали ограждения.
//
// Все детали помечены Purchased = true: они идут в спецификацию и прайс,
// но не в карту раскроя.
// purchasedRailingParts начиная с firstNumber: сквозная нумерация деталей
// пакета. Номера присваивает не decompose (он разбирает тело модели и про
// ограждение не знает), а этот конструктор — иначе Part.Validate() ругается
// на пустой номер.
//
// Место (Placement + Where) — из enggeo.RailingPurchasedLayout: только
// прямой марш со стальным ограждением. Вне его раскладка возвращает nil,
// и детали остаются без места (честное «не знаю», см. Placement.Known).
// Порядок эмиссии обязан совпадать с порядком раскладки один в один:
// расхождение — паника, а не тихий сдвиг номеров (сверяет
// TestPurchasedLayoutZipped).
func purchasedRailingParts(cfg *engineering.StairConfiguration, firstNumber int) []dommfg.Part {
	stepCount := cfg.StepCount
	treadDepth := cfg.TreadDepth.Millimeters()
	stepHeight := cfg.StepHeight.Millimeters()
	// Длина поручня — по оси, из геометрии: та же линия, что строит модель.
	// Горизонтальная проекция здесь запрещена (S-206: −20 % длины).
	railLength := enggeo.RailingRailAxisLen(stepCount, treadDepth, stepHeight)
	sides := enggeo.RailingSideCount(cfg)
	if stepCount <= 0 || railLength <= 0 || sides <= 0 {
		return nil
	}
	// Стойки, фланцы и ригели ставятся на КАЖДУЮ оплачиваемую сторону: при
	// RailingBoth геометрия строит две независимые нитки перил (см.
	// straightRailingSolids/buildLURNailing), и список покупного раньше
	// показывал половину — по одной стороне. Раньше sides всегда был 1,
	// потому что сторона вообще не читалась.
	postsPerSide := railingPostCount(stepCount, treadDepth)
	posts := postsPerSide * sides
	// Длины ригелей по оси — по одному значению на пролёт между стойками.
	// Пролёты бывают неравными, поэтому индекс пролёта идёт по порядку
	// эмиссии (s/r/i), а не усредняется (S-206).
	steps := enggeo.RailingPostSteps(stepCount, treadDepth)
	bayLen := railingBayLengths(steps, treadDepth, stepHeight)

	// Порядок эмиссии (раньше — пять циклов, теперь один список, порядок
	// тот же): поручни, стойки, ригели, фитинги, заглушки.
	var kinds []dommfg.PartKind
	for s := 0; s < sides; s++ {
		kinds = append(kinds, dommfg.PartRailTop)
	}
	for i := 0; i < posts; i++ {
		kinds = append(kinds, dommfg.PartPost)
	}
	// Ригели — два ряда между крайними стойками ОДНОЙ стороны. Пролёт
	// считается по стойкам своей стороны: если взять общее число стоек
	// (postsPerSide × sides), то при двух сторонах пролёт вышел бы вдвое
	// короче настоящего, и в смете лежала бы труба неверной длины.
	// Длина каждого пролёта — по оси (bayLen), пролёты бывают неравными.
	if postsPerSide >= 2 {
		for s := 0; s < sides; s++ {
			for r := 0; r < rigelRows; r++ {
				for i := 0; i < postsPerSide-1; i++ {
					kinds = append(kinds, dommfg.PartRigel)
				}
			}
		}
	}
	// Фитинги — по одному комплекту на каждую стойку.
	for i := 0; i < posts; i++ {
		kinds = append(kinds,
			dommfg.PartTip, dommfg.PartFlange, dommfg.PartSlip, dommfg.PartDeco)
	}
	// Торцевые заглушки — по две на нитку поручня (вход и выход).
	for s := 0; s < sides; s++ {
		kinds = append(kinds, dommfg.PartEndCap, dommfg.PartEndCap)
	}

	layout := enggeo.RailingPurchasedLayout(cfg)
	if layout != nil {
		if len(layout) != len(kinds) {
			panic(fmt.Sprintf("раскладка ограждения: элементов %d, деталей %d — порядок разошёлся",
				len(layout), len(kinds)))
		}
		for i := range kinds {
			if layout[i].Kind != string(kinds[i]) {
				panic(fmt.Sprintf("раскладка ограждения: позиция %d — %q против %q",
					i, layout[i].Kind, kinds[i]))
			}
		}
	}

	out := make([]dommfg.Part, 0, len(kinds))
	// Порядок эмиссии ригелей — s/r/i (сторона, ряд, пролёт), поэтому номер
	// пролёта — это счётчик ригелей по модулю числа пролётов на стороне.
	ri := 0
	for i, kind := range kinds {
		rigelLen := 0.0
		if kind == dommfg.PartRigel && len(bayLen) > 0 {
			rigelLen = bayLen[ri%len(bayLen)]
			ri++
		}
		p := railingPartDims(kind, railLength, rigelLen)
		// Проставляем сквозные номера деталей пакета.
		p.Number = partNumber(kind, firstNumber+i)
		p.Purchased = true
		if layout != nil {
			el := layout[i]
			p.Placement = placePurchased(el, p.Width.Millimeters())
			p.Where = railingWhere(el)
		}
		out = append(out, p)
	}
	return out
}

// railingPartDims — габариты и материал покупной детали по её виду.
// Диаметры и высота стойки — из каталога (переменные файла), длины
// поручня и ригеля — по оси из геометрии (аргументы): резать трубу по
// горизонтальной проекции запрещено (S-206).
func railingPartDims(kind dommfg.PartKind, railLength, rigelLen float64) dommfg.Part {
	switch kind {
	case dommfg.PartRailTop:
		// Перила (поручень) — по одной непрерывной трубе на марш на сторону.
		// При двух сторонах это ДВЕ трубы: одна нитка не может обслуживать обе
		// кромки, геометрия их строит отдельно.
		return dommfg.Part{
			Kind: dommfg.PartRailTop, Material: "STAINLESS-304",
			Thickness: engLen(postWall), Length: engLen(railLength), Width: engLen(postOD),
		}
	case dommfg.PartPost:
		return dommfg.Part{
			Kind: dommfg.PartPost, Material: "STAINLESS-304",
			Thickness: engLen(postWall), Length: engLen(balusterLen), Width: engLen(postOD),
		}
	case dommfg.PartRigel:
		// Ригель — длина своего пролёта по оси (пролёты бывают неравными).
		return dommfg.Part{
			Kind: dommfg.PartRigel, Material: "STAINLESS-304",
			Thickness: engLen(rigelWall), Length: engLen(rigelLen), Width: engLen(rigelOD),
		}
	case dommfg.PartTip:
		// Наконечник регулируемый: овальная площадка под поручень.
		return dommfg.Part{
			Kind: dommfg.PartTip, Material: "STAINLESS-304",
			Thickness: engLen(1.0), Length: engLen(tipSizeL), Width: engLen(tipSizeW),
		}
	case dommfg.PartFlange:
		// Фланец опоры: крепит стойку к ступени.
		return dommfg.Part{
			Kind: dommfg.PartFlange, Material: "STAINLESS-304",
			Thickness: engLen(flangeH), Length: engLen(flangeOD), Width: engLen(flangeOD),
		}
	case dommfg.PartSlip:
		// Накидная часть (муфта).
		return dommfg.Part{
			Kind: dommfg.PartSlip, Material: "STAINLESS-304",
			Thickness: engLen(balusterLen), Length: engLen(slipOD), Width: engLen(slipOD),
		}
	case dommfg.PartDeco:
		// Декоративная накладка (колпачок).
		return dommfg.Part{
			Kind: dommfg.PartDeco, Material: "STAINLESS-304",
			Thickness: engLen(decoH), Length: engLen(decoOD), Width: engLen(decoOD),
		}
	case dommfg.PartEndCap:
		return dommfg.Part{
			Kind: dommfg.PartEndCap, Material: "PLASTIC",
			Thickness: engLen(1.0), Length: engLen(endCapOD), Width: engLen(endCapOD),
		}
	case dommfg.PartStringer, dommfg.PartTread, dommfg.PartRiser, dommfg.PartLanding,
		dommfg.PartHeel, dommfg.PartWeb, dommfg.PartAnchor, dommfg.PartFirstWeb:
		// Детали каркаса сюда попадать не должны: они разбираются из тел
		// модели в decompose, а не строятся здесь. Явный перечень, а не
		// default: линтер exhaustive требует видеть все виды.
		panic(fmt.Sprintf("деталь каркаса %q не покупная", kind))
	default:
		panic(fmt.Sprintf("покупная деталь вида %q не описана в railingPartDims", kind))
	}
}

// placePurchased — Placement из якоря раскладки.
//
// Раскладка даёт опорную точку Ref и оси U/V; ширина W — то же число,
// что лежит в Part.Width (его передаёт вызывающий). Origin подбирается
// так, чтобы Center(L, W) вернулся в якорь (Ref + U·L/2) при ЛЮБОМ L:
// слагаемые с L сокращаются, поэтому длины в сигнатуре нет — это не
// упущение, а математика (unparam здесь прав, и это хорошо).
// Сборочный чертёж ставит номер тем же вызовом Center, поэтому якорь
// и номер совпадают по построению, а не по надежде.
func placePurchased(el enggeo.RailingPlaced, widthMM float64) dommfg.Placement {
	ox := el.Ref.X - el.V.X*(widthMM/2)
	oy := el.Ref.Y - el.V.Y*(widthMM/2)
	oz := el.Ref.Z - el.V.Z*(widthMM/2)
	n := el.U.Cross(el.V)
	nn, ok := n.Normalized()
	if !ok {
		panic(fmt.Sprintf("s-192: вырожденная рамка раскладки у %q", el.Kind))
	}
	vec := func(x, y, z float64) dommfg.Vec3 { return dommfg.Vec3{X: x, Y: y, Z: z} }
	return dommfg.Placement{
		Origin: vec(ox, oy, oz),
		U:      vec(el.U.X, el.U.Y, el.U.Z),
		V:      vec(el.V.X, el.V.Y, el.V.Z),
		N:      vec(nn.X, nn.Y, nn.Z),
		Known:  true,
	}
}

// railingWhere — человеческое место покупной детали: номер на наклейке
// и строка указывают на одну стойку/сторону.
func railingWhere(el enggeo.RailingPlaced) string {
	switch dommfg.PartKind(el.Kind) {
	case dommfg.PartRailTop:
		return fmt.Sprintf("поручень, сторона %s", el.Side)
	case dommfg.PartPost:
		return fmt.Sprintf("стойка %d, сторона %s", el.PostIdx, el.Side)
	case dommfg.PartRigel:
		return fmt.Sprintf("ригель, ряд %d, сторона %s", el.Row+1, el.Side)
	case dommfg.PartTip:
		return fmt.Sprintf("наконечник, стойка %d, сторона %s", el.PostIdx, el.Side)
	case dommfg.PartFlange:
		return fmt.Sprintf("фланец, стойка %d, сторона %s", el.PostIdx, el.Side)
	case dommfg.PartSlip:
		return fmt.Sprintf("муфта, стойка %d, сторона %s", el.PostIdx, el.Side)
	case dommfg.PartDeco:
		return fmt.Sprintf("накладка, стойка %d, сторона %s", el.PostIdx, el.Side)
	case dommfg.PartEndCap:
		return fmt.Sprintf("торец поручня (%s), сторона %s", el.End, el.Side)
	case dommfg.PartStringer, dommfg.PartTread, dommfg.PartRiser, dommfg.PartLanding,
		dommfg.PartHeel, dommfg.PartWeb, dommfg.PartAnchor, dommfg.PartFirstWeb:
		// Детали каркаса место получают в decompose, их Where — whereLabel.
		// Сюда они не доходят; ветка — для полноты switch (exhaustive).
		return el.Kind
	default:
		return el.Kind
	}
}
