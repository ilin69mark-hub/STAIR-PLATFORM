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
	"math"

	"stairplatform/internal/domain/engineering"
	dommfg "stairplatform/internal/domain/manufacturing"
)

// Каталожные размеры покупных деталей ограждения, мм. Дублируют
// engineering.PurchasedParts() осознанно: здесь нужен числовой доступ,
// а каталог — декларация для спецификации и чертежей.
const (
	postOD      = 42.4 // стойка и поручень: полая труба
	postWall    = 1.5
	rigelOD     = 12.0 // ригель: полая трубка
	rigelWall   = 1.0
	balusterLen = 805.7 // высота стойки под поручень
	tipSizeL    = 70.0  // наконечник: овал вдоль поручня
	tipSizeW    = 50.8
	tipSizeH    = 95.4
	flangeOD    = 86.0 // фланец опоры
	flangeH     = 8.0
	slipOD      = 43.0 // накидная часть (муфта)
	decoOD      = 88.0 // декоративная накладка
	decoH       = 17.0
	endCapOD    = 39.2 // торцевая заглушка
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
//	count = ceil((n−1)·b / maxGap) + 1
//	idx_i = 1 + round(i·(n−1)/(count−1))
//
// плюс проверка «ни один промежуток не длиннее maxGap».
//
// Раньше здесь стояла своя формула int(run/1500)+1: на марше 15×270 она
// давала 3 стойки, а геометрия рисовала 4. Спецификация и модель расходились
// — ровно та двойная правда, ради устранения которой и заведён каталог.
func railingPostIndices(n int, b, maxGap float64) []int {
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
func railingPostCount(n int, b, maxGap float64) int {
	return len(railingPostIndices(n, b, maxGap))
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
	railHeight float64, firstNumber int) []dommfg.Part {
	if stepCount <= 0 || railLength <= 0 {
		return nil
	}
	runLength := treadDepth * float64(stepCount-1)
	posts := railingPostCount(stepCount, treadDepth, maxPostSpan)
	var out []dommfg.Part

	// Перила (поручень) — одна непрерывная труба на весь марш.
	out = append(out, dommfg.Part{
		Kind: dommfg.PartRailTop, Material: "STAINLESS-304",
		Thickness: engLen(postWall), Length: engLen(railLength), Width: engLen(postOD),
		Purchased: true,
	})
	// Стойки — по одной на каждой опоре.
	for i := 0; i < posts; i++ {
		out = append(out, dommfg.Part{
			Kind: dommfg.PartPost, Material: "STAINLESS-304",
			Thickness: engLen(postWall), Length: engLen(balusterLen), Width: engLen(postOD),
			Purchased: true,
		})
	}
	// Ригели — два ряда между крайними стойками: posts-1 пролётов в ряд.
	if posts >= 2 {
		span := runLength / float64(posts-1)
		for r := 0; r < rigelRows; r++ {
			for i := 0; i < posts-1; i++ {
				out = append(out, dommfg.Part{
					Kind: dommfg.PartRigel, Material: "STAINLESS-304",
					Thickness: engLen(rigelWall), Length: engLen(span), Width: engLen(rigelOD),
					Purchased: true,
				})
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
	// Торцевые заглушки — две на марш (вход и выход).
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
	_ = railHeight
	// Проставляем сквозные номера деталей пакета.
	for i := range out {
		out[i].Number = partNumber(out[i].Kind, firstNumber+i)
	}
	return out
}
