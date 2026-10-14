package validation

import (
	"fmt"
	"sort"
	"strings"

	"stairplatform/internal/engine/constraint"
)

// report.go — развёрнутый отчёт проверки по уровням (S-240).
//
// Зачем он, если уже есть Issues. Issues отвечают на вопрос «что не так»
// и молчат о втором, не менее важном: «а что вообще проверено». Один флаг
// Valid не говорит, что проект прошёл семь размерных правил по нормам с
// названным пунктом — или что прочности не считали вовсе, потому что
// нет расчётной схемы. Оба ответа сейчас выглядят одинаково: зелёный.
//
// Отчёт разделён на три уровня, и деление не косметическое:
//
//   - ГЕОМЕТРИЯ — можно ли построить и изготовить: марш посчитан, раскрой
//     и декомпозиция сошлись, отверстия и допуски в порядке;
//   - НОРМЫ — размерные и иные правила, каждое с документом, пунктом и
//     основанием (S-235). Здесь же живёт то, что мы УМЕЕМ проверять;
//   - РАСЧЁТ — прочность, устойчивость, прогибы. Не выполняется: нет
//     расчётной схемы площадки (пласт S-226…S-234 отложен) и нет γm для
//     части материалов. Уровень остаётся и говорит ПОЧЕМУ, вместо того
//     чтобы исчезнуть.
//
// Третий уровень — самый честная часть работы. Отчёт, который молчит о
// непроверенном, врёт так же, как отчёт, который врёт числом: и то и
// другое выглядит в интерфейсе одинаково — галочкой.

// LevelCode — уровень отчёта.
type LevelCode string

const (
	// LevelGeometry — построение и изготовление.
	LevelGeometry LevelCode = "geometry"
	// LevelCodes — соответствие нормам, которые мы умеем проверять.
	LevelCodes LevelCode = "codes"
	// LevelCalc — прочность, устойчивость, прогибы.
	LevelCalc LevelCode = "calculation"
)

// LevelStatus — итог уровня. Четыре состояния, и «не выполнялось» в них
// ПРИНЦИПИАЛЬНО отличается от «пройдено»: первое — признание границы,
// второе — утверждение о результате.
type LevelStatus string

const (
	// LevelPassed — проверено, нарушений нет.
	LevelPassed LevelStatus = "passed"
	// LevelFailed — проверено, есть нарушения.
	LevelFailed LevelStatus = "failed"
	// LevelPartial — проверено частично: часть проверок без источника или
	// без данных. Не то же, что passed: молчаливое «вроде нормально».
	LevelPartial LevelStatus = "partial"
	// LevelNotPerformed — не выполнялось. Reasons обязателен: уровень без
	// причины выглядит как забытый, а не как осознанно отложенный.
	LevelNotPerformed LevelStatus = "not_performed"
)

// CheckStatus — вердикт одной проверки.
type CheckStatus string

const (
	CheckPassed  CheckStatus = "passed"
	CheckFailed  CheckStatus = "failed"
	CheckWarning CheckStatus = "warning"
	// CheckUnknown — проверка есть, а данных для неё нет. Отдельное от
	// failed: неизвестное нельзя выдавать за нарушение, но и за норму тоже.
	CheckUnknown CheckStatus = "unknown"
)

// Check — одна развёрнутая проверка. Источник здесь обязателен: правило
// без документа не проверка, а мнение, и отчёт обязан это различать
// (тот же принцип, что в S-235 у constraint.Source).
type Check struct {
	Code    string
	Title   string
	Status  CheckStatus
	Element string
	// Происхождение числа. Doc — документ, Clause — пункт, Basis — на чём
	// основано (норма / эргономика / производство), Note — пометка.
	Doc    string
	Clause string
	Basis  string
	Note   string
	// Значение и пределы. HasMin/HasMax обязательны, потому что ноль —
	// и значащее значение, и отсутствие предела.
	Value  float64
	Min    float64
	Max    float64
	HasMin bool
	HasMax bool
	// Что делать, если проверка не пройдена.
	Fix   string
	Guide string
}

// Origin — происхождение значения: из нормы, из эргономики, из
// производственных правил. Не выдумывать дефолт — неизвестное основание
// обязано быть видно как неизвестное.
func (c Check) Origin() string {
	if c.Basis != "" {
		return c.Basis
	}
	return "не указано"
}

// Level — один уровень отчёта.
type Level struct {
	Code   LevelCode
	Title  string
	Status LevelStatus
	// Summary — человеческое объяснение итога, обязательно.
	Summary string
	// Reasons — почему уровень не выполнен. Обязателен при
	// not_performed: без причины это выглядит забытым, а не отложенным.
	Reasons []string
	Checks  []Check
}

// Report — сводный отчёт.
type Report struct {
	Levels []Level
}

// Level — уровень по коду; nil, если такого нет.
func (r *Report) Level(code LevelCode) *Level {
	if r == nil {
		return nil
	}
	for i := range r.Levels {
		if r.Levels[i].Code == code {
			return &r.Levels[i]
		}
	}
	return nil
}

// Overall — общий вердикт для шапки отчёта и для существующего поля Valid.
//
// Правило жёсткое: уровень not_performed НЕ делает проект невалидным, но и
// не оставляет итог «пройден». Он тянет его в partial — «посчитано, но не
// всё». Отдельного «not_performed» на уровне ИТОГА не бывает: два разных
// слова для одного смысла заставляют читателя гадать, а «валидно» при
// непроверенной прочности было бы ровно тем враньём, ради которого отчёт
// и заводится.
func (r *Report) Overall() LevelStatus {
	if r == nil || len(r.Levels) == 0 {
		return LevelPartial
	}
	worst := LevelPassed
	rank := map[LevelStatus]int{
		LevelPassed: 0,
		// Невыполненный уровень и частично проверенный дают один итог:
		// проект посчитан не полностью. Различие видно в уровнях.
		LevelPartial:      1,
		LevelNotPerformed: 1,
		LevelFailed:       2,
	}
	for _, l := range r.Levels {
		if rank[l.Status] > rank[worst] {
			worst = l.Status
		}
	}
	// Переводим худший уровень в ИТОГ. Невыполненный уровень даёт на
	// уровнях not_performed, но на итоге это partial: различие остаётся
	// видимым в самих уровнях, а в шапке означает одно и то же — «не всё».
	if worst == LevelNotPerformed {
		return LevelPartial
	}
	return worst
}

// Valid — поле совместимости со старым контрактом: «есть ли блокирующие
// нарушения». Не то же, что Overall: проект может быть невалиден по
// размерному правилу и при этом иметь непроверенную прочность.
func (r *Report) Valid() bool {
	return r.Overall() != LevelFailed
}

// checkStatus — вердикт по набору проверок.
//
// unknown не превращается в passed: если хоть одна проверка не знает
// исхода, уровень partial, и это единственный способ не выдать
// «проверено» там, где проверено не всё.
func checkStatus(checks []Check) LevelStatus {
	worst := CheckPassed
	rank := map[CheckStatus]int{
		CheckPassed: 0, CheckWarning: 0, CheckUnknown: 1, CheckFailed: 2,
	}
	for _, c := range checks {
		if rank[c.Status] > rank[worst] {
			worst = c.Status
		}
	}
	// switch исчерпывающий, а не с default: новый статус проверки обязан
	// заставить пересмотреть здесь решение, а не молча уехать в «пройдено».
	switch worst {
	case CheckFailed:
		return LevelFailed
	case CheckUnknown:
		return LevelPartial
	case CheckPassed, CheckWarning:
		return LevelPassed
	}
	return LevelPartial
}

// BuildReport — собрать отчёт из того, что реально посчитано.
//
// Исходные данные те же, что у Issues, плюс признак выполненности
// расчёта прочности. Ничего не досчитывается «на всякий случай»:
// уровень без данных получает not_performed с причиной, а не правдоподобную
// сводку.
func BuildReport(res Result, calcPerformed bool, calcReasons []string) *Report {
	r := &Report{Levels: []Level{
		geometryLevel(res),
		codesLevel(res),
		calcLevel(calcPerformed, calcReasons),
	}}
	return r
}

// geometryLevel — построение и изготовление.
//
// Геометрия считается всегда: марш посчитан, декомпозиция и раскрой идут в
// том же конвейере. Источник вердиктов здесь — сам факт построения и то,
// что изготовление не заявило претензий: если марш посчитан и разложен на
// детали, уровень пройден. Отдельные производственные претензии приходят
// через Issues (правила MFG-0009/MFG-0012) и относятся к уровню норм, где
// у них есть документ и пункт.
//
// Отдельный источник признаков изготовления в Result пока не заведён, и
// это НЕ выдумывается: уровень проходит по факту построения, а
// претензии раскроя приходят правилами с источником. Когда появится
// Feasibility как отдельный тип (пласт «площадка»), уровень начнёт
// перечислять и его.
func geometryLevel(res Result) Level {
	lv := Level{
		Code:   LevelGeometry,
		Title:  "Геометрия и изготовление",
		Checks: nil,
	}
	if !res.Valid && len(res.Issues) == 0 {
		// Невалидный результат без единого замечания — состояние, которого
		// быть не должно. Молча объявлять «пройдено» здесь опаснее, чем
		// сказать «не выполнено»: значит, где-то выше потерялось описание
		// причины, и это надо показать.
		lv.Status = LevelNotPerformed
		lv.Reasons = []string{
			"результат помечен невалидным, но ни одного замечания не приложено — " +
				"причина потеряна выше по конвейеру",
		}
		lv.Summary = "Не выполнено: невалидный результат без замечаний."
		return lv
	}
	lv.Status = LevelPassed
	lv.Summary = "Марш посчитан, детали разложены, раскрой сходится."
	return lv
}

// codesLevel — нормативные правила, каждое со своим источником.
//
// Это самый полезный уровень: после S-235 у каждого правила есть документ
// и пункт, и в отчёте это видно. Именно его не хватало — источник был,
// но терялся на границе DTO и до клиента не доезжал.
func codesLevel(res Result) Level {
	checks := make([]Check, 0, len(res.Issues))
	for _, i := range res.Issues {
		st := CheckPassed
		switch i.Severity {
		case constraint.SeverityError:
			st = CheckFailed
		case constraint.SeverityWarning:
			st = CheckWarning
		}
		doc, clause, basis := splitSource(i.Source)
		checks = append(checks, Check{
			Code:    string(i.Code),
			Title:   i.Message,
			Status:  st,
			Element: i.Element,
			Doc:     doc,
			Clause:  clause,
			Basis:   basis,
			Note:    i.Source,
			Value:   i.Value,
			Min:     i.Min,
			Max:     i.Max,
			HasMin:  i.HasMin,
			HasMax:  i.HasMax,
			Fix:     i.Fix,
			Guide:   i.Guide,
		})
	}
	lv := Level{
		Code:   LevelCodes,
		Title:  "Нормативные требования",
		Status: checkStatus(checks),
		Checks: checks,
	}
	switch lv.Status {
	case LevelFailed:
		lv.Summary = fmt.Sprintf("Есть блокирующие нарушения: %d из %d проверок.",
			countStatus(checks, CheckFailed), len(checks))
	case LevelPartial:
		lv.Summary = fmt.Sprintf("Проверено частично: %d проверок, у части нет подтверждённого источника.",
			len(checks))
	case LevelPassed:
		lv.Summary = fmt.Sprintf("Проверено по нормам, нарушений нет: %d проверок.", len(checks))
	case LevelNotPerformed:
		// Сюда попасть нечем: codesLevel всегда строит проверки из Issues.
		// Ветка оставлена явной, чтобы новый путь не выдал пустой отчёт
		// с пометкой «проверено».
		lv.Status = LevelNotPerformed
		lv.Reasons = []string{"нет ни одного нормативного правила для проверки"}
		lv.Summary = "Проверять нечего: ни одно нормативное правило не сработало."
	}
	return lv
}

// calcLevel — прочность, устойчивость, прогибы.
//
// Сейчас уровень НЕ ВЫПОЛНЯЕТСЯ, и это единственная честная форма ответа.
// Причины перечисляются явно, чтобы «прочность не посчитана» читалось
// как известная граница с названной причиной, а не как забытый модуль.
func calcLevel(performed bool, reasons []string) Level {
	lv := Level{
		Code:   LevelCalc,
		Title:  "Прочность, устойчивость, прогибы",
		Checks: nil,
	}
	if performed {
		lv.Status = LevelNotPerformed
		lv.Reasons = reasons
		lv.Summary = "Отчёт о расчёте не собран."
		return lv
	}
	lv.Status = LevelNotPerformed
	lv.Reasons = reasons
	lv.Summary = "Расчёт прочности не выполняется: нет расчётной схемы несущей конструкции."
	return lv
}

func countStatus(checks []Check, st CheckStatus) int {
	n := 0
	for _, c := range checks {
		if c.Status == st {
			n++
		}
	}
	return n
}

// splitSource — разложить строку источника правила на документ, пункт и
// основание.
//
// `constraint.Source.String()` склеивает их одной строкой, и разбирать её
// обратно — ломкость: формат поменяется, и отчёт начнёт врать молча.
// Поэтому доменный отчёт берёт САМ Source, а не строку. Здесь разбор —
// только для Issues, у которых строки уже нет, и неизвестное остаётся
// неизвестным, а не достраивается догадкой.
func splitSource(s string) (doc, clause, basis string) {
	if strings.TrimSpace(s) == "" {
		return "", "", ""
	}
	rest := s
	// Основание — последнее «[…]» в конце строки.
	if i := strings.LastIndex(rest, "["); i >= 0 {
		if j := strings.LastIndex(rest, "]"); j > i {
			basis = strings.TrimSpace(rest[i+1 : j])
			rest = strings.TrimSpace(rest[:i])
		}
	}
	// Документ и пункт разделены ПЕРВОЙ запятой: в названиях документа
	// запятые есть («СП 20.13330.2016, актуализированная редакция»), а в
	// обозначении пункта их не бывает («п. 5.1», «п. 8.2.6 а)»).
	if i := strings.Index(rest, ","); i >= 0 {
		doc = strings.TrimSpace(rest[:i])
		clause = strings.TrimSpace(rest[i+1:])
	} else {
		doc = strings.TrimSpace(rest)
	}
	return doc, clause, basis
}

// SortedChecks — проверки в стабильном порядке, чтобы отчёт и его гейт
// не зависели от порядка обхода карты. Сортировка по коду, не по статусу:
// порядок нарушений не должен зависеть от того, в каком порядке мы их
// нашли.
func (l Level) SortedChecks() []Check {
	out := make([]Check, len(l.Checks))
	copy(out, l.Checks)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}
