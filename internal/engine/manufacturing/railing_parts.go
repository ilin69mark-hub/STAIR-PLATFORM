// railing_parts.go — ПОКУПНЫЕ детали ограждения для спецификации.
//
// Ограждение не раскраивается из листа: стойки, поручень, ригели и фитинги
// приходят готовыми с завода. Раньше они не попадали в производственный пакет
// вообще — геометрия ограждения строится как декорация (BuildRailingDecor),
// а decompose() разбирает только тело модели. В спецификации ограждения не
// было: ни позиций, ни цен.
//
// Здесь детали ограждения собираются из КОНФИГУРАЦИИ и каталога
// engineering.PurchasedParts(), а не из геометрии: у них постоянные размеры,
// меняется только количество и длина поручня.
//
// Длины и диаметры взяты из каталога internal/domain/engineering/parts.go,
// который сверяется с эталоном FreeCAD (TestCatalog*).

package manufacturing

import (
	"fmt"
	"math"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
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
	// maxPostSpan — максимальный пролёт между стойками, мм (как в эталоне).
	maxPostSpan = 1500.0
	// rigelRows — число рядов ригелей (33% и 66% высоты ограждения).
	rigelRows = 2
)

// railLengthMM — длина поручня вдоль марша: горизонтальный пробег плюс
// вылет за габарит на обоих концах. Вылет 135 мм — как в эталоне
// FreeCAD (OVERHANG); поручень должен перекрывать края марша.
const railOverhangMM = 135.0

func railLengthMM(cfg *engineering.StairConfiguration) float64 {
	run := cfg.TreadDepth.Millimeters() * float64(cfg.StepCount-1)
	l := run + 2*railOverhangMM
	if l <= 0 {
		return 0
	}
	return l
}

// engLen — короткий перевод float64 (мм) в engineering.Length.
func engLen(mm float64) engineering.Length { return engineering.Length(mm) }

// railingPostIndices — индексы ступеней, на которых стоят стойки.
//
// Правило ОБЯЗАТЕЛЬНО совпадает с balusterSteps() в движке геометрии:
//
//	count = ceil((n−1)·b / maxPostSpan) + 1
//	idx_i = 1 + round(i·(n−1)/(count−1))
//
// плюс проверка «ни один промежуток не длиннее maxPostSpan».
//
// Параметра «максимальный пролёт» у функции нет намеренно: значение одно и
// оно равно maxPostSpan. Параметр, который всегда получал одну и ту же
// константу, только провоцировал расхождение — выглядел как настраиваемое
// правило, а правилом не был.
//
// Раньше здесь стояла своя формула int(run/1500)+1: на марше 15×270 она
// давала 3 стойки, а геометрия рисовала 4. Спецификация и модель расходились
// — ровно та двойная правда, ради устранения которой и заведён каталог.
func railingPostIndices(n int, b float64) []int {
	maxGap := maxPostSpan
	if n <= 0 || b <= 0 {
		return nil
	}
	if n == 1 {
		return []int{1}
	}
	count := int(math.Ceil(float64(n-1)*b/maxGap)) + 1
	if count > n {
		count = n
	}
	if count < 2 {
		count = 2
	}
	for {
		idx := make([]int, count)
		for i := 0; i < count; i++ {
			idx[i] = 1 + int(math.Round(float64(i)*float64(n-1)/float64(count-1)))
			if idx[i] < 1 {
				idx[i] = 1
			}
			if idx[i] > n {
				idx[i] = n
			}
		}
		ok := true
		for i := 1; i < len(idx); i++ {
			if float64(idx[i]-idx[i-1])*b > maxGap {
				ok = false
				break
			}
		}
		if ok || count >= n {
			return idx
		}
		count++
	}
}

// railingPostCount — сколько стоек на марше.
func railingPostCount(n int, b float64) int {
	return len(railingPostIndices(n, b))
}

// purchasedRailingParts собирает покупные детали ограждения.
//
// runLength — длина марша по горизонтали (X), мм;
// railLength — длина поручня вдоль марша с учётом вылета, мм;
// railHeight — высота ограждения, мм (из неё считаются ряды ригелей).
//
// Все детали помечены Purchased = true: они идут в спецификацию и прайс,
// но не в карту раскроя.
// purchasedRailingParts начиная с firstNumber: сквозная нумерация деталей
// пакета. Номера присваивает не decompose (он разбирает тело модели и про
// ограждение не знает), а этот конструктор — иначе Part.Validate() ругается
// на пустой номер.
func purchasedRailingParts(stepCount int, treadDepth, railLength,
	railHeight float64, sides int, firstNumber int) []dommfg.Part {
	if stepCount <= 0 || railLength <= 0 || sides <= 0 {
		return nil
	}
	runLength := treadDepth * float64(stepCount-1)
	// Стойки, фланцы и ригели ставятся на КАЖДУЮ оплачиваемую сторону: при
	// RailingBoth геометрия строит две независимые нитки перил (см.
	// straightRailingSolids/buildLURNailing), и список покупного раньше
	// показывал половину — по одной стороне. Раньше sides всегда был 1,
	// потому что сторона вообще не читалась.
	postsPerSide := railingPostCount(stepCount, treadDepth)
	posts := postsPerSide * sides
	var out []dommfg.Part

	// Перила (поручень) — по одной непрерывной трубе на марш на сторону.
	// При двух сторонах это ДВЕ трубы: одна нитка не может обслуживать обе
	// кромки, геометрия их строит отдельно.
	for s := 0; s < sides; s++ {
		out = append(out, dommfg.Part{
			Kind: dommfg.PartRailTop, Material: "STAINLESS-304",
			Thickness: engLen(postWall), Length: engLen(railLength), Width: engLen(postOD),
			Purchased: true,
		})
	}
	// Стойки — по одной на каждой опоре каждой стороны.
	for i := 0; i < posts; i++ {
		out = append(out, dommfg.Part{
			Kind: dommfg.PartPost, Material: "STAINLESS-304",
			Thickness: engLen(postWall), Length: engLen(balusterLen), Width: engLen(postOD),
			Purchased: true,
		})
	}
	// Ригели — два ряда между крайними стойками ОДНОЙ стороны. Пролёт
	// считается по стойкам своей стороны: если взять общее число стоек
	// (postsPerSide × sides), то при двух сторонах пролёт вышел бы вдвое
	// короче настоящего, и в смете лежала бы труба неверной длины.
	if postsPerSide >= 2 {
		span := runLength / float64(postsPerSide-1)
		for s := 0; s < sides; s++ {
			for r := 0; r < rigelRows; r++ {
				for i := 0; i < postsPerSide-1; i++ {
					out = append(out, dommfg.Part{
						Kind: dommfg.PartRigel, Material: "STAINLESS-304",
						Thickness: engLen(rigelWall), Length: engLen(span), Width: engLen(rigelOD),
						Purchased: true,
					})
				}
			}
		}
	}
	// Фитинги — по одному комплекту на каждую стойку.
	for i := 0; i < posts; i++ {
		out = append(out,
			// Наконечник регулируемый: овальная площадка под поручень.
			dommfg.Part{
				Kind: dommfg.PartTip, Material: "STAINLESS-304",
				Thickness: engLen(1.0), Length: engLen(tipSizeL), Width: engLen(tipSizeW),
				Purchased: true,
			},
			// Фланец опоры: крепит стойку к ступени.
			dommfg.Part{
				Kind: dommfg.PartFlange, Material: "STAINLESS-304",
				Thickness: engLen(flangeH), Length: engLen(flangeOD), Width: engLen(flangeOD),
				Purchased: true,
			},
			// Накидная часть (муфта).
			dommfg.Part{
				Kind: dommfg.PartSlip, Material: "STAINLESS-304",
				Thickness: engLen(balusterLen), Length: engLen(slipOD), Width: engLen(slipOD),
				Purchased: true,
			},
			// Декоративная накладка (колпачок).
			dommfg.Part{
				Kind: dommfg.PartDeco, Material: "STAINLESS-304",
				Thickness: engLen(decoH), Length: engLen(decoOD), Width: engLen(decoOD),
				Purchased: true,
			},
		)
	}
	// Торцевые заглушки — по две на нитку поручня (вход и выход).
	for s := 0; s < sides; s++ {
		out = append(out,
			dommfg.Part{
				Kind: dommfg.PartEndCap, Material: "PLASTIC",
				Thickness: engLen(1.0), Length: engLen(endCapOD), Width: engLen(endCapOD),
				Purchased: true,
			},
			dommfg.Part{
				Kind: dommfg.PartEndCap, Material: "PLASTIC",
				Thickness: engLen(1.0), Length: engLen(endCapOD), Width: engLen(endCapOD),
				Purchased: true,
			},
		)
	}
	_ = railHeight
	// Проставляем сквозные номера деталей пакета.
	for i := range out {
		out[i].Number = partNumber(out[i].Kind, firstNumber+i)
	}
	return out
}
