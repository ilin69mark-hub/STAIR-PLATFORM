// Package geometry implements the Geometry Engine (ENG-0002, ENG-GEO-0007):
// it builds parametric B-Rep models of stair flights from a StairConfiguration.
// The engine knows only the domain model and the domain-agnostic geometry
// kernel; parameters are the single source of truth of geometry (BC-002).
package geometry

import (
	"context"
	"fmt"
	"math"
	"strings"

	"stairplatform/internal/domain/engineering"
	"stairplatform/internal/engine/scheduler"
	kerngeo "stairplatform/internal/geometry"
)

// flightSideInsetMM — величина, на которую каждый марш П-образной лестницы
// (платформенный режим) сужается от внешней кромки. Оба марша сужаются на
// одну и ту же величину, поэтому между ними образуется внутренний зазор
// 2·flightSideInsetMM = 100 мм. Площадка остаётся шириной 2·W — зазор
// получается внутренним за счёт сужения маршей, внешний габарит не меняется.
const flightSideInsetMM = 50.0

// StringerExtent возвращает вертикальный габарит профиля косоура
// прямого марша: от уровня пола (низ передней гранки) до верхнего
// седла пилы (H − StepThickness). Экспортируется, чтобы советник и
// раскрой использовали единый габарит косоура (BC-002).
func StringerExtent(heightMm, stepThMm float64) float64 {
	return heightMm - stepThMm
}

// maxPlateFrameMM — верхняя граница толщины листового металла, по которой
// каркас опознаётся как пластинчатый. Нужна для автоназначения материала,
// когда код материала не задан: косоур из стали тонкий, косоур из дерева —
// 40–100 мм (это же правило уже записано в
// engineering.StairConfiguration.Material).
//
// Значение держится вровень с максимальной строкой норматива на листовой
// косоур (engineering.PlateMaxThicknessMM = 12 мм, решение владельца
// 2026-10-05). Пока оно было 10 мм, стальной косоур 12 мм при незаданном
// материале опознавался бы как ДЕРЕВО и ушёл бы в другую геометрию.
const maxPlateFrameMM = engineering.PlateMaxThicknessMM

// StringerPlateDepthMM — глубина боковой пластины косоура металлокаркаса по
// нормали к линии подъёма. Копия решения Ниоры (niora.ru/calculator/metal и
// их цех): косоур — ровная наклонная пластина, ступени лежат на её верхнем
// ребре, торец проступи упирается в боковую грань, на пол пластина встаёт
// передней вертикальной гранью. Прежняя гребёнка (посадочные места на
// уровне низа проступи, спинка шириной в толщину металла) давала в 3D
// «кашу из непонятных элементов»: две тонкие пилы внутри ширины марша, по
// которым ступень стоит как на рельсах.
const StringerPlateDepthMM = 300.0

// plateStringer — металлический каркас (косоур боковой пластиной) или нет.
func plateStringer(cfg *engineering.StairConfiguration) bool {
	if m := cfg.Material; m != "" {
		return strings.HasPrefix(m, "STEEL") || strings.HasPrefix(m, "METAL")
	}
	// Материал не задан — автоназначение по толщине детали (тот же вывод,
	// что в раскрое: тонкая деталь из листа, толстая — пиломатериал).
	return cfg.StringerThickness.Millimeters() <= maxPlateFrameMM
}

// PlateStringerFrame — признак пластинчатого (металлического) каркаса.
// Экспортируется для раскроя: заготовка боковой пластины (полоса по склону
// марша шириной «глубина пластины + толщина») и заготовка гребёнки (полоса
// шириной в шаг ступени) считаются по-разному.
func PlateStringerFrame(cfg *engineering.StairConfiguration) bool {
	return plateStringer(cfg)
}

// BuildStraightFlight строит параметрическую B-Rep модель прямого марша
// (ENG-GEO-0007). Модель состоит из 2 косоуров и n проступей при
// StepThickness > 0 (EDR-0004); подступенки (3n тел) добавляются только при
// cfg.Riser — при false лестница имеет открытые ступени. Итого при
// StepThickness > 0 и Riser: 2+4n тела.
// Координаты: X — направление подъёма, Y — ширина, Z — высота (ADR-0008).
// Геометрия всегда вычисляется заново из параметров (BC-002).
// BuildStraightFlight строит прямой марш с производственной точностью
// тесселяции. Облегчённый меш для отображения строится отдельным проходом
// (GeneratePreviewMesh), а не этим вызовом: числа из облегчённой сборки в
// расчёт не идут.
func BuildStraightFlight(cfg *engineering.StairConfiguration) (*kerngeo.Compound, error) {
	return buildStraightFlight(cfg, ProductionQuality)
}

func buildStraightFlight(cfg *engineering.StairConfiguration, q Quality) (*kerngeo.Compound, error) {
	if err := validateFlight(cfg); err != nil {
		return nil, err
	}
	w := cfg.Width.Millimeters()
	b := cfg.TreadDepth.Millimeters()
	h := cfg.StepHeight.Millimeters()
	n := cfg.StepCount
	t := cfg.StringerThickness.Millimeters()
	st := cfg.StepThickness.Millimeters()

	// Build-циклы параллелятся (B3, EDR-0034 §3.2): каждое тело строится
	// независимо (Extrude чистый), порядок в Compound фиксирован индексами
	// слотов (косоуры → проступи → подступенки) — детерминизм ADR-0003.
	builds := make([]func() (*kerngeo.Solid, error), 0, 4*n+2)

	// Металлокаркас: косоур — боковая пластина ПО КРАЯМ ширины, и ступени
	// идут между пластинами ([t, w−t]): торец проступи упирается в боковую
	// грань косоура, как на Ниоре. Дерево: прежняя гребёнка внутри ширины,
	// ступень кладётся на седло пилы и перекрывает косоур по всей ширине.
	plate := plateStringer(cfg)
	yA, yC := w/4-t/2, 3*w/4-t/2
	// Ширина ступеней и подступенков — вся ширина марша [0, w] и для
	// гребёнки, и для металла: свеса за косоуры нет (решение владельца).
	y0, y1 := 0.0, w
	if plate {
		yA, yC = 0, w-t
		// Ступень и подступенок идут ВО ВСЮ ШИРИНУ, включая плоскости
		// косоуров (чертёж 2026-10-05): свеса за косоуры нет. Раньше детали
		// уводились внутрь на plateJoinMM против z-fighting — вместо этого
		// смещение ролей в 3D задаёт фронтенд (ROLE_DEPTH_BIAS).
		y0, y1 = 0.0, w
	}
	sp := cfg.CombOrDefault()
	// Металлокаркас: косоур-гребёнка (чертёж 2026-10-05) — посадочная линия
	// ступеней идёт сверху, между посадками зубья, спина StringerSpine по
	// нормали. Прежняя ровная пластина заменена: она не держала ступень.
	// Дерево: прежняя гребёнка внутри ширины, ступень кладётся на седло пилы.
	buildProfile := func(y float64) []kerngeo.Point3 {
		if plate {
			p, err := combStringerProfile(n, b, h, st, sp.StringerSpine)
			if err != nil {
				// Ошибку поднимет сборка ниже; здесь профиль-заглушка,
				// чтобы не терять сигнал в замыкании build-функции.
				return []kerngeo.Point3{
					kerngeo.NewPoint3(0, y, 0), kerngeo.NewPoint3(b, y, 0),
					kerngeo.NewPoint3(b, y, h), kerngeo.NewPoint3(0, y, h),
				}
			}
			return shiftY(p, y)
		}
		return stringerProfile(n, b, h, st, y, t)
	}
	for i, y := range []float64{yA, yC} {
		i, y := i, y
		builds = append(builds, func() (*kerngeo.Solid, error) {
			profile := buildProfile(y)
			solid, err := kerngeo.Extrude(profile, kerngeo.NewVector3(0, 1, 0), t)
			if err != nil {
				return nil, fmt.Errorf("geometry: stringer %d: %w", i, err)
			}
			return solid.WithRole("stringer"), nil
		})
	}

	if st <= kerngeo.Precision {
		return buildCompound(builds)
	}

	// проступи во всю ширину [0,w]. Верх на уровне носика (k+1)·h, толщина st
	// вниз (EDR-0004): низ проступи ложится на седло пилы косоура. Проступь
	// глубже шага на st (охват [k·b−st, (k+1)·b]): её задняя кромка нависает
	// над подступенком, который выдвинут на столько же (см. ниже).
	//
	// Скругление носа (филёнка) задаётся радиусом, который приходит из
	// материала ступеней (StairConfiguration.TreadNoseRadiusMM; у металла ноль).
	// Из-за него проступь собирается иначе, чем раньше: сечение строится в
	// плоскости XZ и вытягивается вдоль ширины (+Y), как косоур. Прежний
	// вариант экструдировал горизонтальный прямоугольник вверх по Z, и нос в
	// такой модели — острый угол бокса, который нельзя скруглить без
	// изменения всей конструкции тела.
	noseR := noseRadiusFor(cfg)
	// СТОРОНА ФАСКИ ПРОСТУПИ (решение владельца 2026-10-08): филёнка R10
	// ставится на сторону СВЕСА, а не на верхнюю кромку со стороны подъёма.
	//
	// Что это значит в координатах. Проступь k занимает по X
	// [k·b−treadRear, k·b−treadRear+treadDepth], подступенок k — [k·b−rt, k·b].
	// Поэтому свес — это кромка ПРИ min X (x0): проступь выходит за спину
	// подступенка на (treadRear − rt) и нависает над ней. Фаска должна стоять
	// именно там.
	//
	// Это совпадает с утверждённым чертежом: freecad-out/ГЕОМЕТРИЯ.md — «R10
	// срезает верхнюю десятку ЗАДНЕЙ грани» и «сверху задняя грань ступени
	// срезана скруглением R10» (высота касания 30, а не 40). То есть расходился
	// код, а не чертёж.
	//
	// Раньше стоял roundFront — фаска попадала на кромку при max X (x1), то
	// есть на ту сторону, где проступь упирается в зуб косоура и где свеса
	// нет. Владелец увидел это как «фаска повёрнута в сторону косоура».
	// Объём среза при этом не меняется (TestTreadNoseRadiusRemovesQuarterRound
	// остаётся в силе) — меняется только сторона.
	treadRoundSide := roundBack
	// Геометрия узла по чертежу: подступенок стоит в плоскости зуба и имеет
	// толщину rt, ступень нависает над его задней гранью на нос CombFrame.
	// Nosing, поэтому задняя кромка ступени на k·b − (rt + nosing), а глубина
	// ступени = шаг + нос.
	rtFrame := cfg.RiserThickness.Millimeters()
	if rtFrame <= kerngeo.Precision {
		rtFrame = st
	}
	// Для дерева геометрия прежняя: ступень глубже шага на толщину ступени и
	// нависает над подступенком. Для металлокаркаса — по чертежу: подступенок
	// в плоскости зуба, ступень нависает на нос.
	treadRear, treadDepth := st, b+st
	if plate {
		treadRear = rtFrame + sp.NosingLength
		treadDepth = b + sp.NosingLength
	}
	// МАРШ БЕЗ ПОДСТУПЕНКА (решение владельца 2026-10-08).
	//
	// Свес ступени назад (treadRear) существует затем, чтобы нависать над
	// подступенком. Подступенок убрали — и свес надо перенести, иначе две
	// беды сразу:
	//
	//  1) свес 60 мм (металл: rt 40 + нос 20) и 40 мм (дерево: st) остаётся
	//     над пустотой высотой в подъём — «место, которое занимал подступенок,
	//     пустое»;
	//  2) сдвинуть ступень вперёд, уперев заднюю грань в зуб, НЕЛЬЗЯ:
	//     тогда передняя кромка уезжает за плоскость зуба следующей ступени
	//     и ступень ВРЕЗАЕТСЯ в тело косоура (проверено: 120 вершин проступи
	//     внутри гребёнки, глубина 20 мм на металле и 40 мм на дереве).
	//
	// Правильная посадка: передняя кромка ступени должна приходиться ровно на
	// плоскость зуба следующей ступени (x = (k+1)·b), дальше — нельзя, там
	// тело косоура. Значит глубина не меняется, а ступень отодвигается назад
	// ровно настолько, насколько она длиннее седла:
	//
	//	treadRear = treadDepth − b
	//
	// Металл: 290 − 270 = 20 мм (это и есть нос). Дерево: 310 − 270 = 40 мм
	// (толщина ступени). Свес уходит на ЗАДНЮЮ кромку и становится ровно тем
	// концом, который раньше закрывал подступенок, — носом. Пустоты под
	// подступенком больше нет: под свесом остаётся нормальная тень от носа.
	if !cfg.Riser {
		treadRear = treadDepth - b
	}
	for k := 0; k < n; k++ {
		k := k
		builds = append(builds, func() (*kerngeo.Solid, error) {
			x0, x1 := float64(k)*b-treadRear, float64(k)*b-treadRear+treadDepth
			if plate && k == n-1 {
				// Последняя ступень доходит до конца косоура: не хватало
				// CombFrame.TopStepExtend (решение владельца 2026-10-05).
				x1 = float64(n) * b
			}
			z := float64(k+1)*h - st
			section, err := plateSection(q, x0, x1, z, st, noseR, treadRoundSide)
			if err != nil {
				return nil, fmt.Errorf("geometry: tread %d: %w", k, err)
			}
			solid, err := kerngeo.Extrude(shiftY(section, y0), kerngeo.NewVector3(0, 1, 0), y1-y0)
			if err != nil {
				return nil, fmt.Errorf("geometry: tread %d: %w", k, err)
			}
			return solid.WithRole("tread"), nil
		})
	}

	// подступенки (только при cfg.Riser, ENG-GEO-0007): вертикальное
	// полотно во всю ширину [0,w], толщиной по X. Классический подступенок:
	// верхний край упирается в нижнюю плоскость вышележащей проступи
	// ((k+1)·h − st), нижний — на нижележащую ступень (k·h), спина — в
	// вертикальный сброс косоура. Проступи глубже шага на st, поэтому
	// подступенок k выдвинут навстречу подъёму: охват [k·b−st, k·b] кладёт
	// своё дно целиком на проступь k−1 (она доходит до k·b), верх — под
	// проступь k, а спина (x=k·b) — на сброс гребёнки. Косоуры в этом
	// окне лежат ниже низа подступенка (седло на z=k·h−st), поэтому полотно
	// единое, без пазов: одно тело на ступень. Внизу марша (k=0) подступенок
	// стоит на полу перед передней гранью косоура — лицевая панель марша
	// [−st, 0] вместе со свесом первой проступи, спиной (x=0) вплоткую к
	// передней грани. При выключенном флаге ступени открытые.
	//
	// Раньше здесь стоял ранний выход — из-за него марш без подступенка терял
	// ещё и МОНТАЖНЫЕ ПЛАСТИНЫ (решение владельца 2026-10-08): пятки пола и
	// выхода нужны независимо от того, есть подступенок или нет.
	rt := rtFrame
	for k := 0; k < n; k++ {
		if !cfg.Riser {
			break
		}
		k := k
		x0 := float64(k)*b - rt
		z0, z1 := float64(k)*h, float64(k+1)*h-st
		if plate {
			// Металлокаркас: высота подступенка — весь подъём ступени, он
			// закрывает проём от низа нижележащей ступени до низа этой.
			// У первой ступени низ на полу.
			z0 = float64(k)*h - st
			if z0 < 0 {
				z0 = 0
			}
		}
		builds = append(builds, func() (*kerngeo.Solid, error) {
			profile := []kerngeo.Point3{
				kerngeo.NewPoint3(x0, y0, z0),
				kerngeo.NewPoint3(x0, y0, z1),
				kerngeo.NewPoint3(x0, y1, z1),
				kerngeo.NewPoint3(x0, y1, z0),
			}
			solid, err := kerngeo.Extrude(profile, kerngeo.NewVector3(1, 0, 0), rt)
			if err != nil {
				return nil, fmt.Errorf("geometry: riser %d: %w", k, err)
			}
			return solid.WithRole("riser"), nil
		})
	}

	// ПЯТКИ ВЫРЕЗА (чертёж 2026-10-05). Только для металлокаркаса: у дерева
	// обоймы нет — там ступень лежит прямо на косоуре. Порядок слотов
	// фиксирован (пятка 1 → пятка 2 → пол → выход → пластина 1-й ступени),
	// по каждому косоуру: детерминизм ADR-0003.
	//
	// БЕЗ ПОДСТУПЕНКА (решение владельца 2026-10-08) пятка 2 — НЕ ставится.
	// Она одевает вертикальный срез зуба и работает в паре с подступенком
	// (вместе они дают Г-образную обойму выреза); подступенка нет — обоймы
	// нет, и пятка 2 висит в воздухе. Остальные четыре ставятся как обычно:
	// пятка 1 держит ступень на седле, пятка пола крепит косоур к полу, пятка
	// выхода — к перекрытию, пластина 1-й ступени закрывает первый вырез.
	//
	// Роль у пятки 2 — "web", и её исключение из геометрии автоматически
	// убирает её из спецификации, раскроя и цены: перечень деталей строится
	// по телам модели (manufacturing.roleKind), отдельного «не считать» не
	// нужно и не появится рассинхрон.
	if !plate {
		return buildCompound(builds)
	}
	for _, side := range []struct {
		id  string
		hy0 float64
	}{
		{"A", t},                    // у внутренней грани косоура A
		{"B", w - t - sp.HeelWidth}, // у внутренней грани косоура B
	} {
		side := side
		for k := 0; k < n; k++ {
			k := k
			builds = append(builds, func() (*kerngeo.Solid, error) {
				return combHeelSolid(cfg, sp, k, side.hy0)
			})
		}
		// Пятка 2 — только вместе с подступенком.
		if cfg.Riser {
			for k := 0; k < n-1; k++ {
				k := k
				builds = append(builds, func() (*kerngeo.Solid, error) {
					return combWebSolid(cfg, sp, k, side.hy0)
				})
			}
		}
		builds = append(builds,
			func() (*kerngeo.Solid, error) { return combAnchorSolid(q, cfg, sp, side.hy0) },
			func() (*kerngeo.Solid, error) { return combTopAnchorSolid(q, cfg, sp, side.hy0) },
		)
		// Пластина 1-й ступени — подкладка ПЕРВОГО ПОДСТУПЕНКА (решение
		// владельца 2026-10-08): вертикальная пластина в плоскости x = 0, с
		// высотой ровно между пяткой пола и первой посадкой. К ней
		// приваривается подступенок 0 (он стоит в плоскости x ∈ [−rt, 0]),
		// и она же закрывает первый вырез.
		//
		// Подступенка нет — держать нечего, и пластина остаётся висеть в
		// воздухе перед косоуром. Ставим её только вместе с подступенком.
		if cfg.Riser {
			builds = append(builds,
				func() (*kerngeo.Solid, error) { return combFirstWebSolid(cfg, sp, side.hy0) },
			)
		}
	}

	return buildCompound(builds)
}

// noseRadiusFor — радиус скругления носка для конфигурации, ограниченный
// толщиной детали. Радиус приходит из материала ступеней (у металла нулевой),
// а больше толщины он быть не может: нос не поместится в деталь.
func noseRadiusFor(cfg *engineering.StairConfiguration) float64 {
	r := cfg.TreadNoseRadiusMM.Millimeters()
	if st := cfg.StepThickness.Millimeters(); r > st {
		return st
	}
	return r
}

// buildCompound выполняет все build-замыкания параллельно через Scheduler
// (B3, EDR-0034) и собирает Compound в порядке слотов. Ошибка — первый по
// индексу сбой либо отмена контекста (внутренний Background: отмена не
// требуется на стадии построения).
func buildCompound(builds []func() (*kerngeo.Solid, error)) (*kerngeo.Compound, error) {
	solids := make([]*kerngeo.Solid, len(builds))
	err := scheduler.New(0).Execute(context.Background(), len(builds), func(i int) error {
		s, e := builds[i]()
		if e != nil {
			return e
		}
		solids[i] = s
		return nil
	})
	if err != nil {
		return nil, err
	}
	return kerngeo.NewCompound(solids...), nil
}

// BuildLShapeFlight строит параметрическую B-Rep модель L-образной
// лестницы (EDR-0005, ENG-GEO-0007): нижний прямой марш (n1 ступеней),
// горизонтальная площадка на высоте H1 и верхний прямой марш (n2
// ступеней), повёрнутый на 90° по горизонтали. Координаты: X — направление
// подъёма нижнего марша, Y — его ширина, Z — высота (ADR-0008); верхний
// марш идёт вдоль +Y. Геометрия всегда вычисляется заново из параметров
// (BC-002). Роли тел проставляются для корректной декомпозиции.
func BuildLShapeFlight(cfg *engineering.StairConfiguration) (*kerngeo.Compound, error) {
	return buildLShapeFlight(cfg, ProductionQuality)
}

func buildLShapeFlight(cfg *engineering.StairConfiguration, q Quality) (*kerngeo.Compound, error) {
	if err := validateFlight(cfg); err != nil {
		return nil, err
	}
	if cfg.Flight != engineering.FlightLShape {
		return nil, fmt.Errorf("geometry: configuration flight must be l_shape")
	}
	w := cfg.Width.Millimeters()
	b := cfg.TreadDepth.Millimeters()
	h := cfg.StepHeight.Millimeters()
	st := cfg.StepThickness.Millimeters()
	n1 := cfg.LowerStepCount
	n2 := cfg.StepCount - n1
	wp := cfg.LandingWidth.Millimeters()
	// Глубина площадки (вдоль нижнего марша, X в плане). При 0 — квадратная
	// (равна ширине марша W).
	ld := cfg.LandingDepth.Millimeters()
	if ld <= 0 {
		ld = w
	}
	// EDR-0005 §4.5: H1 = n1·h — уровень площадки.
	h1 := float64(n1) * h

	// нижний марш в локальных координатах (без поворота).
	lowerModel, err := buildStraightFlight(subFlight(cfg, h1, n1), q)
	if err != nil {
		return nil, fmt.Errorf("geometry: lower flight: %w", err)
	}

	// верхний марш: строится как прямой в локальных координатах, затем
	// поворот на 90° вокруг Z (направление подъёма → вдоль +Y) и перенос
	// так, чтобы марш начинался с края площадки на высоте H1.
	upperModel, err := buildStraightFlight(subFlight(cfg, float64(n2)*h, n2), q)
	if err != nil {
		return nil, fmt.Errorf("geometry: upper flight: %w", err)
	}
	// Площадка и ориентация маршей зависят от направления поворота
	// (CONF-DIRECTION). Правосторонний поворот (по умолчанию): площадка в
	// плане [L1, L1+W]×[0, Wp], верхний марш — поворотом на +90° вокруг Z
	// (подъём → +Y), занимает [L1, L1+W]×[Wp, Wp+L2] на высоте H1.
	// Левосторонний (TurnLeft) — зеркальная в плане компоновка: площадка
	// [0, W]×[0, Wp] слева, нижний марш повёрнут на 180° и поднимается по
	// −X (его верх стыкуется с правой гранью площадки x = W), верхний марш
	// повёрнут на +90° и поднимается вдоль +Y от края площадки.
	l1 := float64(n1) * b
	left := cfg.Direction == engineering.TurnLeft
	landingX0 := l1
	lowerTransform := kerngeo.Identity()
	upperTransform := kerngeo.Translate(l1+w, wp, h1).Mul(kerngeo.RotateZ(math.Pi / 2))
	if left {
		landingX0 = w - ld
		lowerTransform = kerngeo.Translate(w+l1, w, 0).Mul(kerngeo.RotateZ(math.Pi))
		upperTransform = kerngeo.Translate(w, wp, h1).Mul(kerngeo.RotateZ(math.Pi / 2))
	}

	// площадка: горизонтальная плита толщиной st на высоте H1, план
	// [landingX0, landingX0+Ld]×[0, Wp] (EDR-0005 §4.8), роль "landing".
	noseR := noseRadiusFor(cfg)
	landing, err := buildLanding(q, ld, wp, landingX0, h1, st, landingRoundSide(left), noseR)
	if err != nil {
		return nil, err
	}

	// нижний марш: для правого поворота — без поворота; для левого —
	// повёрнут на 180° (левосторонняя компоновка). Порядок тел сохранён
	// (нижний марш → площадка → верхний марш) для детерминизма.
	solids := make([]*kerngeo.Solid, 0, len(lowerModel.Solids())+1+len(upperModel.Solids()))
	for _, s := range lowerModel.Solids() {
		solids = append(solids, kerngeo.TransformSolid(s, lowerTransform))
	}
	solids = append(solids, landing)
	for _, s := range upperModel.Solids() {
		solids = append(solids, kerngeo.TransformSolid(s, upperTransform))
	}
	return kerngeo.NewCompound(solids...), nil
}

// subFlight создаёт конфигурацию прямого марша-секции (нижний/верхний
// марш многомаршевой лестницы): высота и число ступеней переопределяются,
// остальные параметры наследуются от родительской конфигурации.
func subFlight(cfg *engineering.StairConfiguration, height float64, steps int) *engineering.StairConfiguration {
	return &engineering.StairConfiguration{
		Width:             cfg.Width,
		Height:            engineering.Length(height),
		Length:            cfg.Length,
		Angle:             cfg.Angle,
		Flight:            engineering.FlightStraight,
		StepCount:         steps,
		StepHeight:        cfg.StepHeight,
		StepWidth:         cfg.StepWidth,
		TreadDepth:        cfg.TreadDepth,
		Clearance:         cfg.Clearance,
		RailingHeight:     cfg.RailingHeight,
		StringerLength:    engineering.Length(float64(steps) * cfg.TreadDepth.Millimeters()),
		StringerThickness: cfg.StringerThickness,
		StepThickness:     cfg.StepThickness,
		// Толщина подступенка наследуется: секция марша — та же деталь из
		// того же материала, что и весь марш. Без этой строки L/U-марш молча
		// получил бы 0 и откатился на толщину ступени.
		RiserThickness: cfg.RiserThickness,
		Riser:          cfg.Riser,
		// Материал секции ОБЯЗАТЕЛЬНО наследуется: по нему выбирается тип
		// косоура — боковая пластина (metal, 3–8 мм) или пила-гребёнка
		// (дерево, 30–60 мм). Раньше материал в под-конфигурацию не попадал,
		// и выбор шёл по толщине: у Г- и П-образного марша с листом 8 мм
		// пластина строилась, а при заданном материале и толщине 50 мм —
		// уже гребёнка, тогда как раскрой смотрел на материал ВСЕЙ
		// лестницы и считал пластину. Геометрия и раскрой расходились.
		Material:      cfg.Material,
		TreadMaterial: cfg.TreadMaterial,
	}
}

// subFlightWidth — то же, что subFlight, но с явно заданной шириной марша
// (переопределяет cfg.Width). Используется П-образной лестницей в
// платформенном режиме, чтобы сузить марши на flightSideInsetMM и получить
// внутренний зазор, не затрагивая веерный режим (subFlight).
func subFlightWidth(cfg *engineering.StairConfiguration, height float64, steps int, width engineering.Length) *engineering.StairConfiguration {
	c := subFlight(cfg, height, steps)
	c.Width = width
	return c
}

// BuildUShapeFlight строит параметрическую B-Rep модель П-образной
// лестницы (EDR-0006, ENG-GEO-0007). Поворот между маршами выбирается
// полем cfg.TurnKind: площадка (TurnPlatform, по умолчанию) либо поворотные
// ступени (TurnWinder). Диспетчеризация по TurnKind сохраняет обратную
// совместимость: пустой/неизвестный TurnKind трактуется как площадка.
func BuildUShapeFlight(cfg *engineering.StairConfiguration) (*kerngeo.Compound, error) {
	return buildUShapeFlight(cfg, ProductionQuality)
}

func buildUShapeFlight(cfg *engineering.StairConfiguration, q Quality) (*kerngeo.Compound, error) {
	if err := validateFlight(cfg); err != nil {
		return nil, err
	}
	if cfg.Flight != engineering.FlightUShape {
		return nil, fmt.Errorf("geometry: configuration flight must be u_shape")
	}
	if cfg.TurnKind == engineering.TurnWinder {
		return buildUShapeWinderFlight(cfg, q)
	}
	return buildUShapePlatform(q, cfg)
}

// uShapePlatformTransforms возвращает трансформы нижнего и верхнего маршей и
// габариты площадки П-образной лестницы с площадкой (платформенный режим).
// Верхний марш развёрнут на 180° относительно нижнего (подъём в
// противоположную сторону); в левом варианте (TurnLeft) раскладка зеркальна
// в плане. Те же самые трансформы используют перила (buildLURNailing), поэтому
// рассинхрон между ступенями и перилами исключён по построению.
func uShapePlatformTransforms(cfg *engineering.StairConfiguration) (lowerT, upperT kerngeo.Transform, landingX0, landingY float64) {
	w := cfg.Width.Millimeters()
	b := cfg.TreadDepth.Millimeters()
	h := cfg.StepHeight.Millimeters()
	n1 := cfg.LowerStepCount
	h1 := float64(n1) * h
	l1 := float64(n1) * b
	landingY = 2 * w
	// Зазор 100 мм между маршами: каждый марш сужается на flightSideInsetMM
	// от внешней кромки (wEff = W − flightSideInsetMM); площадка остаётся
	// 2·W, зазор получается внутренним.
	wEff := w - flightSideInsetMM
	left := cfg.Direction == engineering.TurnLeft
	landingX0 = l1
	lowerT = kerngeo.Identity()
	// Верхний марш — подъём в противоположную сторону (180°): правый вариант
	// развёрнут на RotateZ(π) и само вращение сужает марш до [W+50, 2W];
	// левый — зеркален и поднимается по +X из левого края площадки, занимая
	// Y∈[W+50, 2W]. Нижний (левый) — Y∈[0, wEff]. Оба варианта примыкают к
	// площадке той же кромкой, что и раньше (узел поворота — в одном углу
	// площадки, а не классический switchback через всю площадку).
	upperT = kerngeo.Translate(l1, 2*w, h1).Mul(kerngeo.RotateZ(math.Pi))
	if left {
		landingX0 = 0
		lowerT = kerngeo.Translate(w+l1, wEff, 0).Mul(kerngeo.RotateZ(math.Pi))
		upperT = kerngeo.Translate(w, w+flightSideInsetMM, h1)
	}
	return
}

// buildUShapePlatform строит П-образную лестницу с площадкой (EDR-0006 §4):
// нижний прямой марш (n1), горизонтальная площадка на высоте H1 и верхний
// прямой марш (n2), параллельный нижнему и развёрнутый на 180°. Площадка
// соединяет оба марша на одном Z-уровне H1 и охватывает полную ширину
// 2·W (внешние кромки обоих маршей), будучи по ширине равной двум пролётам.
func buildUShapePlatform(q Quality, cfg *engineering.StairConfiguration) (*kerngeo.Compound, error) {
	w := cfg.Width.Millimeters()
	h := cfg.StepHeight.Millimeters()
	st := cfg.StepThickness.Millimeters()
	n1 := cfg.LowerStepCount
	n2 := cfg.StepCount - n1
	// Трансформы маршей и габариты площадки вычисляются одной функцией и
	// переиспользуются перилами (uShapePlatformTransforms), чтобы перила
	// гарантированно совпадали со ступенями (EDR-0006 §4.8).
	lowerTransform, upperTransform, landingX0, landingY := uShapePlatformTransforms(cfg)
	// EDR-0006 §4.5: H1 = n1·h — уровень площадки.
	h1 := float64(n1) * h
	// Марши сужаются на flightSideInsetMM (внутренний зазор 100 мм), площадка
	// остаётся 2·W (см. uShapePlatformTransforms).
	wEff := w - flightSideInsetMM

	lowerModel, err := buildStraightFlight(subFlightWidth(cfg, h1, n1, engineering.Length(wEff)), q)
	if err != nil {
		return nil, fmt.Errorf("geometry: lower flight: %w", err)
	}
	upperModel, err := buildStraightFlight(subFlightWidth(cfg, float64(n2)*h, n2, engineering.Length(wEff)), q)
	if err != nil {
		return nil, fmt.Errorf("geometry: upper flight: %w", err)
	}

	// площадка: плита толщиной st на высоте H1, план
	// [landingX0, landingX0+W]×[0, 2W], роль "landing".
	noseR := noseRadiusFor(cfg)
	landing, err := buildLanding(q, w, landingY, landingX0, h1, st,
		landingRoundSide(cfg.Direction == engineering.TurnLeft), noseR)
	if err != nil {
		return nil, err
	}

	solids := make([]*kerngeo.Solid, 0, len(lowerModel.Solids())+1+len(upperModel.Solids()))
	for _, s := range lowerModel.Solids() {
		solids = append(solids, kerngeo.TransformSolid(s, lowerTransform))
	}
	solids = append(solids, landing)
	for _, s := range upperModel.Solids() {
		solids = append(solids, kerngeo.TransformSolid(s, upperTransform))
	}
	return kerngeo.NewCompound(solids...), nil
}

// winderArcRelTol — допуск сектора поворотной ступени в ДОЛЯХ радиуса (S-172).
//
// Абсолютный допуск здесь бессмыслен: заготовка веерной ступени вырезается,
// отклонение на радиусе 1250 мм неразличимо глазом, а на радиусе 5 мм то же
// отклонение было бы грубой ошибкой формы. Поэтому допуск ОТНОСИТЕЛЬНЫЙ.
//
// Угол сектора — не полуоборот, а π/nw (180°, делённые на число
// поворотных ступеней), см. buildWinders: dth = π/nw. Значение подобрано так,
// чтобы при типовых nw = 3 сегментов осталось 8 — столько же, сколько давала
// прежняя константа arcSegments. Проверено: при радиусе 1250 мм отклонение
// многоугольника от дуги 3,4 мм (было 5,35 мм при тех же 8 сегментах на
// радиусе 1250 — то есть точность выросла).
//
// Сделать сектор глаже — теперь одна правка этого числа.
const winderArcRelTol = 0.0027

// sectorRing — контур сектора кольца [r, R] на угле от a0 до a1 в плоскости
// z = z0: внешняя дуга вперёд, внутренняя назад. Основа веерной ступени
// П-образного марша с поворотом на 180° (BuildUShapeWinderFlight).
func sectorRing(q Quality, r, R, a0, a1, z0 float64) []kerngeo.Point3 {
	segs := kerngeo.ArcSegmentsRelative(R, a1-a0, q.relTol(winderArcRelTol))
	pts := make([]kerngeo.Point3, 0, 2*segs+2)
	for i := 0; i <= segs; i++ {
		a := a0 + (a1-a0)*float64(i)/float64(segs)
		pts = append(pts, kerngeo.NewPoint3(R*math.Cos(a), R*math.Sin(a), z0))
	}
	// Внутренняя дуга считается от своего радиуса r: при относительном
	// допуске сегментов на ней столько же, сколько на внешней, и контур
	// остаётся замкнутым по числу точек с обеих сторон.
	innerSegs := kerngeo.ArcSegmentsRelative(r, a1-a0, q.relTol(winderArcRelTol))
	for i := 0; i <= innerSegs; i++ {
		a := a1 - (a1-a0)*float64(i)/float64(innerSegs)
		pts = append(pts, kerngeo.NewPoint3(r*math.Cos(a), r*math.Sin(a), z0))
	}
	return pts
}

// BuildUShapeWinderFlight строит П-образную лестницу с поворотными
// ступенями (EDR-0006 §4, поворот на 180°): нижний прямой марш (n1),
// набор из nw поворотных ступеней (веер на 180° в просвете шириной Wp
// между маршами) и верхний прямой марш (n2 = n − n1 − nw). Поворотные
// ступени наследуют высоту h и проступь b прямых маршей; каждая поднимает
// на h, общий подъём марша H = n·h. Роль тел: "winder" для поворотных ступеней.
func BuildUShapeWinderFlight(cfg *engineering.StairConfiguration) (*kerngeo.Compound, error) {
	return buildUShapeWinderFlight(cfg, ProductionQuality)
}

func buildUShapeWinderFlight(cfg *engineering.StairConfiguration, q Quality) (*kerngeo.Compound, error) {
	if err := validateFlight(cfg); err != nil {
		return nil, err
	}
	if cfg.Flight != engineering.FlightUShape {
		return nil, fmt.Errorf("geometry: configuration flight must be u_shape")
	}
	if cfg.TurnKind != engineering.TurnWinder {
		return nil, fmt.Errorf("geometry: configuration turn kind must be winder")
	}
	w := cfg.Width.Millimeters()
	b := cfg.TreadDepth.Millimeters()
	h := cfg.StepHeight.Millimeters()
	st := cfg.StepThickness.Millimeters()
	n1 := cfg.LowerStepCount
	nw := cfg.WinderCount
	n2 := cfg.StepCount - n1 - nw
	if n2 < 1 {
		return nil, fmt.Errorf("geometry: upper flight must have at least 1 step (n − n1 − nw = %d)", n2)
	}
	wp := cfg.LandingWidth.Millimeters() // ширина просвета между маршами
	h1 := float64(n1) * h
	l1 := float64(n1) * b
	left := cfg.Direction == engineering.TurnLeft

	lowerModel, err := buildStraightFlight(subFlight(cfg, h1, n1), q)
	if err != nil {
		return nil, fmt.Errorf("geometry: lower flight: %w", err)
	}
	upperModel, err := buildStraightFlight(subFlight(cfg, float64(n2)*h, n2), q)
	if err != nil {
		return nil, fmt.Errorf("geometry: upper flight: %w", err)
	}

	// Внутренние ребра стыка маршей с поворотом (нижний верх /
	// верхний низ), через которые поворотные ступени соединяют марши.
	var pLower, pUpper kerngeo.Point3
	lowerTransform := kerngeo.Identity()
	upperTransform := kerngeo.Translate(l1, w+wp+w, h1+float64(nw)*h).Mul(kerngeo.RotateZ(math.Pi))
	if left {
		pLower = kerngeo.NewPoint3(w, w, h1)
		pUpper = kerngeo.NewPoint3(0, wp, h1+float64(nw)*h)
		lowerTransform = kerngeo.Translate(w+l1, w, 0).Mul(kerngeo.RotateZ(math.Pi))
		upperTransform = kerngeo.Translate(0, w+wp, h1+float64(nw)*h).Mul(kerngeo.RotateZ(math.Pi))
	} else {
		pLower = kerngeo.NewPoint3(l1, w, h1)
		pUpper = kerngeo.NewPoint3(l1, w+wp, h1+float64(nw)*h)
	}

	winders, err := buildWinders(q, w, h, st, n1, nw, pLower, pUpper)
	if err != nil {
		return nil, err
	}

	solids := make([]*kerngeo.Solid, 0, len(lowerModel.Solids())+len(winders)+len(upperModel.Solids()))
	for _, s := range lowerModel.Solids() {
		solids = append(solids, kerngeo.TransformSolid(s, lowerTransform))
	}
	solids = append(solids, winders...)
	for _, s := range upperModel.Solids() {
		solids = append(solids, kerngeo.TransformSolid(s, upperTransform))
	}
	return kerngeo.NewCompound(solids...), nil
}

// buildWinders строит nw поворотных ступеней — веер на 180° между
// внутренними ребрами стыка pLower (нижний марш) и pUpper (верхний марш).
// Веер центрируется в середине отрезка pLower–pUpper; внутренний радиус ri
// достигает внутренних рёбер обоих маршей, внешний ro = ri + W — их внешних
// кромок (EDR-0006 §4.8). Каждая ступень — сектор кольца, выдавленный по Z
// на толщину st, верх на высоте H1 + (k+1)·h (общий подъём h). Роль "winder".
func buildWinders(q Quality, w, h, st float64, n1, nw int, pLower, pUpper kerngeo.Point3) ([]*kerngeo.Solid, error) {
	if nw < 3 {
		return nil, fmt.Errorf("geometry: winder count must be at least 3")
	}
	h1 := float64(n1) * h
	ox, oy := (pLower.X+pUpper.X)/2, (pLower.Y+pUpper.Y)/2
	ux, uy := pLower.X-ox, pLower.Y-oy
	ul := math.Hypot(ux, uy)
	if ul < kerngeo.Precision {
		return nil, fmt.Errorf("geometry: degenerate winder pivot")
	}
	ux, uy = ux/ul, uy/ul
	ri := ul     // расстояние от центра веера до внутренних рёбер (половина просвета)
	ro := ri + w // до внешних кромок маршей
	a0 := math.Atan2(uy, ux)
	dth := math.Pi / float64(nw)

	sols := make([]*kerngeo.Solid, 0, nw)
	for k := 0; k < nw; k++ {
		aa := a0 + float64(k)*dth
		ab := aa + dth
		zTop := h1 + float64(k+1)*h
		profile := sectorRing(q, ri, ro, aa, ab, zTop-st)
		solid, err := kerngeo.Extrude(profile, kerngeo.NewVector3(0, 0, 1), st)
		if err != nil {
			return nil, fmt.Errorf("geometry: winder %d: %w", k, err)
		}
		solid = kerngeo.TransformSolid(solid, kerngeo.Translate(ox, oy, 0))
		sols = append(sols, solid.WithRole("winder"))
	}
	return sols, nil
}

// noseArcToleranceMM — допуск скругления носа проступи, мм (S-172).
//
// Нос — единственная кривая на проступи, которую видно вблизи и по которой
// идёт блик, поэтому допуск здесь жёстче остальных: 0,035 мм. При типовом
// радиусе R = 10 это даёт 10 сегментов на четверть окружности — столько же,
// сколько было константой, и отклонение 0,031 мм (блик на кромке не
// «ломается»). Но стоит поменять радиус скругления — число сегментов
// пересчитается само, а останется константой, заданная для другого радиуса.
const noseArcToleranceMM = 0.035

// pushDistinct добавляет точку в контур, если она не совпадает с последней
// добавленной. Профили скруглённой проступи в предельных случаях (радиус равен
// толщине) вырождают касательные в углы, и без этой проверки контур получает
// повторяющиеся вершины — а такое тело не manifold.
func pushDistinct(section []kerngeo.Point3, p kerngeo.Point3) []kerngeo.Point3 {
	if n := len(section); n > 0 && section[n-1].Distance(p) <= kerngeo.Precision*10 {
		return section
	}
	return append(section, p)
}

// roundSide — какая верхняя кромка плиты скругляется: +1 — по x = x1
// (передняя), −1 — по x = x0 (задняя), 0 — не скруглять.
type roundSide int

const (
	roundNone  roundSide = 0
	roundFront roundSide = 1
	roundBack  roundSide = -1
)

// plateSection строит сечение плиты в плоскости XZ для вытягивания вдоль
// ширины: прямоугольник x0..x1, z..z+thickness, у которого ОДИН верхний угол
// скруглён дугой радиуса r. Таких плит две — проступь (нос по x1) и площадка
// (внешняя кромка, см. landingRoundSide), поэтому хелпер общий.
//
// Точки перечислены В ПОРЯДКЕ ОБХОДА КОНТУРА: снизу вверх по левому краю,
// вдоль низа, вверх по правому краю, вдоль верха, снова вниз. Порядок важен
// не для красоты: он определяет ориентацию граней тела, а несогласованный
// контур (например, если начать с угла и вернуться назад) даёт
// «polygon is not simple» в триангуляции.
//
// При r ≤ 0 или side == roundNone возвращается обычный прямоугольник — то же
// тело, что и до появления фасок, поэтому металлическая лестница не меняется
// ни на миллиметр.
func plateSection(q Quality, x0, x1, zBottom, thickness, r float64, side roundSide) ([]kerngeo.Point3, error) {
	zTop := zBottom + thickness
	pt := func(x, z float64) kerngeo.Point3 { return kerngeo.NewPoint3(x, 0, z) }

	if r <= kerngeo.Precision || side == roundNone {
		return []kerngeo.Point3{pt(x0, zBottom), pt(x1, zBottom), pt(x1, zTop), pt(x0, zTop)}, nil
	}
	// Точка касания не должна уйти за противоположную грань: по боковой — это
	// r ≤ толщины (при r = толщина нос становится полукруглым и боковая грань
	// схлопывается в точку, что pushDistinct и обрабатывает), по верхней —
	// r ≤ длины плиты.
	if r > thickness+kerngeo.Precision || r > (x1-x0)+kerngeo.Precision {
		return nil, fmt.Errorf("geometry: plate rounding radius %v does not fit %vx%v plate", r, x1-x0, thickness)
	}

	section := []kerngeo.Point3{pt(x0, zBottom), pt(x1, zBottom)}
	switch side {
	case roundFront:
		// Идём вверх по правой грани, дуга в левый верхний угол, затем по верху
		// назад к x0. Дуга идёт ОТ грани К верху — так соответствует обходу.
		arc, err := kerngeo.RoundCorner(pt(x1, zBottom), pt(x1, zTop), pt(x0, zTop), r,
			kerngeo.ArcSegments(r, math.Pi/2, q.arcTol(noseArcToleranceMM)))
		if err != nil {
			return nil, fmt.Errorf("geometry: plate corner rounding: %w", err)
		}
		section = pushDistinct(section, pt(x1, zTop-r))
		for _, p := range arc {
			section = pushDistinct(section, p)
		}
		section = pushDistinct(section, pt(x1-r, zTop))
	case roundBack:
		// Симметрично: вверх по правой грани до угла, по верху влево, дуга в
		// левый верхний угол, вниз к x0.
		//
		// Точки касания (x0+r, zTop) и (x0, zTop−r) добавляются ЯВНО: дуга
		// приходит полушаговыми вершинами вписанного многоугольника и в свои
		// концы не попадает.
		//
		// Последней точкой обхода здесь НЕ является (x0, zTop): дуга уже
		// пришла в (x0, zTop−r), и добавление (x0, zTop) возвращало срезанный
		// угол и делало полигон «не простым» — вертикальный отрезок x0 шёл
		// вверх до zTop и тут же обратно вниз. Из-за этого roundFront был
		// единственной рабочей стороной, а roundBack не вызывался нигде.
		arc, err := kerngeo.RoundCorner(pt(x1, zTop), pt(x0, zTop), pt(x0, zBottom), r,
			kerngeo.ArcSegments(r, math.Pi/2, q.arcTol(noseArcToleranceMM)))
		if err != nil {
			return nil, fmt.Errorf("geometry: plate corner rounding: %w", err)
		}
		section = pushDistinct(section, pt(x1, zTop))
		section = pushDistinct(section, pt(x0+r, zTop))
		for _, p := range arc {
			section = pushDistinct(section, p)
		}
		section = pushDistinct(section, pt(x0, zTop-r))
		// Обход закрывается последней точкой сечения (x0, zBottom) — она уже
		// стоит в начале, добавлять (x0, zTop) нельзя.
		//
		// Вырожденный случай r == толщина: нос становится полукруглым, боковая
		// грань схлопывается в точку, и дуга приходит РОВНО в первую точку
		// обхода. Последняя вершина тогда совпадает с первой — замыкать на неё
		// нельзя, полигон получится «не простым». Последнюю вершину снимаем.
		if n := len(section); n > 1 && section[n-1].Distance(section[0]) <= kerngeo.Precision*10 {
			section = section[:n-1]
		}
		return section, nil
	}
	section = pushDistinct(section, pt(x0, zTop))
	return section, nil
}

// buildLanding строит твёрдое тело горизонтальной прямоугольной плиты
// площадки толщиной st с верхней гранью на уровне h1 (EDR-0005 §4.8):
// план [x0, x0+w] × [0, Wp] — ширина марша W вдоль +X, ширина площадки
// Wp вдоль +Y.
// buildLanding строит площадку — плиту x0..x0+w × 0..wp толщиной st с
// верхом на уровне h1.
//
// side задаёт, какая из двух кромок вдоль X скругляется: та, что свободна.
// Кромка y = wp всегда занята верхним маршем, а кромка y = 0 идёт вдоль
// внешнего края поворота, но она перпендикулярна скругляемой, и две
// перпендикулярные скруглённые кромки полиэдральная модель (Solid из плоских
// граней) не выражает: для этого нужны две независимые операции над рёбрами.
// Поэтому скругляется одна — та, что видна из комнаты.
func buildLanding(q Quality, w, wp, x0, h1, st float64, side roundSide, r float64) (*kerngeo.Solid, error) {
	if r > st {
		r = st
	}
	section, err := plateSection(q, x0, x0+w, h1-st, st, r, side)
	if err != nil {
		return nil, fmt.Errorf("geometry: landing: %w", err)
	}
	solid, err := kerngeo.Extrude(section, kerngeo.NewVector3(0, 1, 0), wp)
	if err != nil {
		return nil, fmt.Errorf("geometry: landing: %w", err)
	}
	return solid.WithRole("landing"), nil
}

// landingRoundSide определяет, какая кромка площадки свободна. Марши
// примыкают к площадке с одной стороны по X, поэтому вторая кромка (дальняя
// от поворота) — внешняя, и её скругляют.
func landingRoundSide(left bool) roundSide {
	if left {
		return roundBack
	}
	return roundFront
}

func validateFlight(cfg *engineering.StairConfiguration) error {
	if cfg == nil {
		return fmt.Errorf("geometry: configuration is required")
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("geometry: %w", err)
	}
	if cfg.StepHeight.Millimeters() <= 0 {
		return fmt.Errorf("geometry: step height must be positive")
	}
	if cfg.TreadDepth.Millimeters() <= 0 {
		return fmt.Errorf("geometry: tread depth must be positive")
	}
	if cfg.Width.Millimeters() <= 2*cfg.StringerThickness.Millimeters() {
		return fmt.Errorf("geometry: width must exceed two stringer thicknesses")
	}
	return nil
}

// shiftY сдвигает профиль по ширине: сечение ступени строится в плоскости
// XZ при y = 0, а между боковыми пластинами косоура она должна лечь от y0.
func shiftY(pts []kerngeo.Point3, y0 float64) []kerngeo.Point3 {
	if y0 == 0 {
		return pts
	}
	out := make([]kerngeo.Point3, len(pts))
	for i, p := range pts {
		out[i] = kerngeo.NewPoint3(p.X, p.Y+y0, p.Z)
	}
	return out
}

// plateStringerProfile строит профиль боковой пластины косоура
// металлокаркаса в плоскости XZ — копия решения Ниоры.
//
// Плоскость профиля фиксирована (y=0); параметр yOff ушёл, когда выяснилось,
// что все вызовы передавали ноль, а держать ради этого «гибкость» значило
// держать неиспользуемый параметр.
//
// Форма: ровная наклонная полоса ПОСТОЯННОЙ глубины depth по нормали к
// линии подъёма, на всём марше. Верхнее ребро — посадочная линия ступеней:
// прямая через нижние углы проступей (k·b, (k+1)·h − st). Нижнее ребро —
// та же линия, сдвинутая на depth по нормали вниз. Оба торца вертикальны:
// передний на x = 0 (от пола до первой посадки), задний — на последней
// посадке (n−1)·b. Пятая вершина — пересечение нижнего ребра с полом.
//
// ПОЧЕМУ БЕЗ ПОЛКИ СВЕРХУ: с горизонтальной полкой до x = n·b верх
// получался тоньше марша (254 мм против 300 по нормали) — пластина
// «заужалась» к верху, и это было видно в 3D. Теперь полоса везде
// одинаковой глубины, а сверху её накрывает отдельная горизонтальная
// площадка (см. landingProfile) — ровно как у Ниоры.
// plateProfilePlaneY — координата плоскости профиля пластины по Y.
const plateProfilePlaneY = 0.0

func plateStringerProfile(n int, b, h, st, depth float64) []kerngeo.Point3 {
	// Глубина пластины отсчитывается ПО НОРМАЛИ к линии подъёма: это и есть
	// «300 мм пластины» из чертежа. Раньше сдвиг нижнего ребра был задан
	// ВЕРТИКАЛЬНЫМ отрезком depth·b/L, и перпендикулярная глубина выходила
	// depth·(b/L)² — то есть не 300 мм, а 216 при подъёме 175, 233 при 150
	// и 167 при 250. Спецификация не выполнялась ни при одном угле, и
	// «глубина» дышала вместе с наклоном марша.
	//
	// Задний срез остаётся ВЕРТИКАЛЬНЫМ (как на Ниоре), поэтому
	// вертикальный сдвиг равен depth/cos(угла) = depth·L/b, и
	// перпендикулярное расстояние между посадочной и нижней линиями
	// равно ровно depth при любом угле подъёма.
	drop := depth
	if b > kerngeo.Precision {
		drop = depth * math.Hypot(b, h) / b
	}
	firstZ := h - st // посадка первой ступени
	// Посадочная линия идёт от первой посадки (0, h−st) с уклоном подъёма
	// h/b. На двухступенчатом и длиннее марше её конец — последняя посадка
	// ((n−1)·b, n·h−st). На ОДНОСТУПЕНЧАТОМ марше посадка одна, линия
	// вырождается в точку, и профиль из пяти вершин склеивался бы в
	// отрезок («profile points are collinear»). Поэтому для n < 2 берём
	// половину шага: линия остаётся той же наклонной прямой, только короче.
	seatDX := float64(n-1) * b
	if seatDX <= kerngeo.Precision {
		seatDX = b / 2
	}
	lastSeatX := seatDX
	topZ := firstZ + seatDX*h/b
	// Задний срез не уходит под пол и не делает контур самопересекающимся:
	// пятка должна остаться левее среза. На коротком марше (1–3 ступени)
	// глубина 300 мм физически не помещается, и срез обрезается по маршу —
	// 1% запаса оставлены на саму пятку, иначе нижнее ребро ложится ровно
	// на задний срез и профиль перестаёт быть простым многоугольником.
	if maxDrop := firstZ + lastSeatX*0.99*h/b; drop > maxDrop {
		drop = maxDrop
	}
	floorX := 0.0
	if h > kerngeo.Precision {
		floorX = (drop - firstZ) * b / h
	}
	if floorX < 0 {
		floorX = 0
	}
	return []kerngeo.Point3{
		kerngeo.NewPoint3(0, plateProfilePlaneY, 0),
		kerngeo.NewPoint3(0, plateProfilePlaneY, firstZ),
		kerngeo.NewPoint3(lastSeatX, plateProfilePlaneY, topZ),
		kerngeo.NewPoint3(lastSeatX, plateProfilePlaneY, topZ-drop),
		kerngeo.NewPoint3(floorX, plateProfilePlaneY, 0),
	}
}

// PlateBlankMM — габарит боковой пластины в системе координат, где ось
// вдоль склона: длина и ширина заготовки лазерного раскроя.
//
// Считается из ТОГО ЖЕ профиля, что и геометрия, поэтому деталь в раскрое
// совпадает с телом в меше. Раньше раскрой брал горизонтальный габарит
// марша (run) и «глубина + толщина»: для марша 4480×2800 это давало
// 4480×308 против фактических 5024×300 — деталь короче геометрии на 544 мм
// и шире её на 8 мм, то есть CAD-деталь не совпадала с моделью.
func PlateBlankMM(n int, b, h, st, depth float64) (length, width float64) {
	pts := plateStringerProfile(n, b, h, st, depth)
	L := math.Hypot(b, h)
	if L <= kerngeo.Precision {
		return 0, 0
	}
	// Ось вдоль склона (b, h)/L и нормаль вниз (h, −b)/L.
	ux, uz := b/L, h/L
	mx, mz := h/L, -b/L
	sMin, sMax := math.Inf(1), math.Inf(-1)
	pMin, pMax := math.Inf(1), math.Inf(-1)
	for _, p := range pts {
		s := p.X*ux + p.Z*uz
		q := p.X*mx + p.Z*mz
		sMin, sMax = math.Min(sMin, s), math.Max(sMax, s)
		pMin, pMax = math.Min(pMin, q), math.Max(pMax, q)
	}
	return sMax - sMin, pMax - pMin
}

// stringerProfile строит строго простой профиль косоура-гребёнки в
// плоскости XZ при y = yOff: пила с посадочными местами на уровне низа
// проступи ((k+1)·h − st) и вертикальными сбросами h на границах ступеней
// (седло = низ проступи, ENG-GEO-0007). Низ доски — спинка, параллельная
// линии посадочных мест на расстоянии t (StringerThickness) по нормали;
// сзади (у верхней ступени) — перпендикулярный срез из головы пилы,
// спереди — упор в пол и вертикальная передняя грань (контур начинается
// с передней грани). Вершины: (0, 0), (0, h−st), седло/сброс, …,
// (n·b, n·h−st), P_top, F.
func stringerProfile(n int, b, h, st, yOff, t float64) []kerngeo.Point3 {
	pts := make([]kerngeo.Point3, 0, 2*n+3)
	pts = append(pts,
		kerngeo.NewPoint3(0, yOff, 0),
		kerngeo.NewPoint3(0, yOff, h-st),
	)
	for k := 0; k < n-1; k++ {
		pts = append(pts,
			kerngeo.NewPoint3(float64(k+1)*b, yOff, float64(k+1)*h-st),
			kerngeo.NewPoint3(float64(k+1)*b, yOff, float64(k+2)*h-st),
		)
	}
	pts = append(pts, kerngeo.NewPoint3(float64(n)*b, yOff, float64(n)*h-st))

	// Спинка: линия посадочных мест, сдвинутая на толщину t по нормали
	// вниз (поперёк марша в плоскости XZ). Верхний срез вертикален
	// (перпендикулярен полу, как нижний), спинка упирается в пол (z = 0).
	L := math.Hypot(b, h)
	pTop := kerngeo.NewPoint3(
		float64(n)*b, // верхний срез вертикален (x = n·b)
		yOff,
		float64(n)*h-st-t*b/L, // z = n·h − st − t·b/L
	)
	fx := pTop.X - pTop.Z/h*b
	if fx < 0 {
		fx = 0
	}
	if fx > float64(n)*b {
		fx = float64(n) * b
	}
	pts = append(pts,
		pTop,
		kerngeo.NewPoint3(fx, yOff, 0),
	)
	return pts
}
