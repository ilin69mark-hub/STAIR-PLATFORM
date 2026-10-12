package geometry

import "math"

// Допуск тесселяции (S-172).
//
// Проблема, которую это решает. Число сегментов дуги задавалось константами
// в билдерах: 8 для сектора поворотной ступени, 10 для скругления носа,
// 12 для овального паза, 24 для сечения трубки, 4 для углов профиля
// поручня. Константа не зависит ни от радиуса, ни от материала, ни от того,
// насколько деталь чувствительна к форме. Сменили радиус — получили другую
// погрешность, и заметить это нельзя: метр вообще нигде не записан.
//
// Математика. Дуга радиуса R, разбитая на n сегментов по углу θ, отклоняется
// от идеальной окружности (вписанная хорда против дуги) на
//
//	d = R · (1 − cos(θ / 2n))
//
// Отсюда минимальное n при допуске d ≤ tol:
//
//	n = ⌈θ / (2 · arccos(1 − tol/R))⌉
//
// Ceil, а не округление: округление дало бы n на сегмент меньше и
// превысило бы допуск. Лучше лишний сегмент, чем тихо невыполненное
// требование к точности.

// DefaultArcToleranceMM — допуск по умолчанию, если вызывающий не задал
// свой. В подобран так, чтобы декоративные элементы (профили ограждения)
// выглядели гладко, а не съедали детализацию.
const DefaultArcToleranceMM = 0.05

// ArcSegments возвращает число сегментов, достаточное чтобы отклонение
// многоугольника от идеальной дуги не превышало tolerance мм.
//
// radius — радиус дуги, мм;
// sweep — угол дуги в радианах (неотрицательный);
// tolerance — допуск отклонения, мм; ноль или отрицательное значение
// подставляет DefaultArcToleranceMM.
//
// Гарантии:
//   - результат не меньше MinArcSegments — дуга из двух отрезков выглядит
//     углом, а дугой перестаёт быть;
//   - отклонение результата ≤ tolerance (с точностью до float64).
func ArcSegments(radius, sweep, tolerance float64) int {
	return ArcSegmentsMin(radius, sweep, tolerance, MinArcSegments)
}

// MinArcSegments — нижняя граница числа сегментов, меньше которой дуга не
// читается глазом как дуга.
const MinArcSegments = 4

// ArcSegmentsMin — то же, что ArcSegments, но с явной нижней границей.
// Нужна там, где деталь почти плоская и геометрически хватило бы двух
// сегментов, но силуэт должен остаться округлым.
func ArcSegmentsMin(radius, sweep, tolerance float64, minSegments int) int {
	if tolerance <= 0 {
		tolerance = DefaultArcToleranceMM
	}
	if minSegments < 1 {
		minSegments = 1
	}
	if radius <= 0 || sweep <= 0 {
		return minSegments
	}
	// Допуск больше радиуса: arccos(1 − tol/R) не определён. В этом случае
	// геометрия «круглая настолько, что точность не важна» — берём минимум.
	ratio := tolerance / radius
	if ratio >= 1 {
		return minSegments
	}
	denom := 2 * math.Acos(1-ratio)
	if denom <= 0 {
		return minSegments
	}
	n := int(math.Ceil(sweep / denom))
	if n < minSegments {
		return minSegments
	}
	return n
}

// ArcChordDeviationMM — фактическое отклонение многоугольника от дуги при
// заданном числе сегментов. Нужно тестам и проверкам: показывает, что
// ArcSegments действительно уложилась в допуск, а не просто вернула число.
func ArcChordDeviationMM(radius, sweep float64, segments int) float64 {
	if segments < 1 || radius <= 0 {
		return 0
	}
	return radius * (1 - math.Cos(sweep/(2*float64(segments))))
}

// ArcSegmentsRelative — то же, что ArcSegments, но допуск задан ДОЛЕЙ
// радиуса, а не миллиметрами.
//
// Нужен там, где абсолютная точность бессмысленна. Пример: сектор
// поворотной ступени — заготовка веерной ступени. Отклонение в пол-метра
// на радиусе 300 мм неразличимо и в геометрии, и на раскрое, и на цене;
// то же отклонение на радиусе 5 мм уже было бы грубой ошибкой формы.
// Относительный допуск выражает именно эту мысль: «доля радиуса».
//
// При relTol ≤ 0 берётся MinArcSegments.
func ArcSegmentsRelative(radius, sweep, relTol float64) int {
	if relTol <= 0 || radius <= 0 {
		return MinArcSegments
	}
	return ArcSegments(radius, sweep, radius*relTol)
}
