package constraint

import (
	"fmt"
	"strings"

	"stairplatform/internal/domain/engineering"
)

// profiles.go — нормативные профили по назначению здания (S-236, п. 67
// библии нормативов).
//
// Почему профилей несколько: нормы на лестницы разные для жилых,
// общественных и производственных зданий, а проверять их одним набором
// нельзя. Почему профили НЕ различают числами: подтверждённых значений
// по назначению у нас пока нет, и выдумывать их (например, «1000 мм
// ограждения в общественном здании») опаснее, чем сделать порог общим.
// Что профили делают РЕАЛЬНО: превращают ограждение из предупреждения в
// блокировку (S-239) и помечают правила источником назначения.
//
// Механизм версий уже был (одна активная версия на код) — новых сущностей
// не заводим, профиль это ConstraintSet с конкретным набором правил.

// Profiles — все профили по назначению (для гейтов и UI-перечисления).
func Profiles() []string {
	return []string{
		string(engineering.PurposeHouse),
		string(engineering.PurposeFlat),
		string(engineering.PurposePublic),
		string(engineering.PurposeIndustrial),
	}
}

// ProfileFor возвращает нормативный профиль по назначению здания.
//
// Дефолт (пустое или неизвестное значение) — дом, СП 55: решение владельца
// 2026-10-13. Профиль STANDARD (v1, заморожен) остаётся доступен как
// есть — из него профили и строятся.
func ProfileFor(p engineering.BuildingPurpose) *ConstraintSet {
	purpose := p.PurposeEffective()
	set := StandardProfile(strings.ToUpper(string(purpose)))

	// Ограждение: в жило/общественно/производственном здании 900 мм — не
	// «подсказка», а нижняя граница: предупреждением брак не поймать
	// (S-239). В STANDARD v1 остаётся warning — профиль заморожен.
	if c, ok := set.Active(SAF_RAILING_HEIGHT); ok {
		c.Severity = SeverityError
		c.Message = "высота ограждения должна быть не менее 900 мм"
		c.Fix = "увеличьте высоту ограждения — предупреждение не пропускает расчёт"
		c.Advice = &ConstraintAdvice{
			Param: "Высота перил",
			Guide: fmt.Sprintf(
				"Высота ограждения {value} мм, нужно не менее {min} мм. "+
					"Профиль %s (%s): предел 900 мм — с источником %s",
				purposeProfileName(purpose), purposeNormDoc(purpose), c.Source.Doc),
			Suggest: false,
		}
		if c.Source != nil {
			c.Source = &Source{
				Doc:    c.Source.Doc,
				Clause: c.Source.Clause,
				Basis:  c.Source.Basis,
				Note: c.Source.Note + "; профиль " + string(purpose) +
					": нарушение блокирует расчёт (S-239)",
			}
		}
	}
	return set
}

func purposeProfileName(p engineering.BuildingPurpose) string {
	switch p {
	case engineering.PurposeHouse:
		return "одноквартирный дом"
	case engineering.PurposeFlat:
		return "многоквартирный жилый дом"
	case engineering.PurposePublic:
		return "общественное здание"
	case engineering.PurposeIndustrial:
		return "производственное здание"
	}
	return "одноквартирный дом"
}

func purposeNormDoc(p engineering.BuildingPurpose) string {
	switch p {
	case engineering.PurposeHouse:
		return "СП 55.13330"
	case engineering.PurposeFlat:
		return "СП 54.13330"
	case engineering.PurposePublic:
		return "СП 118.13330"
	case engineering.PurposeIndustrial:
		return "СП 56.13330"
	}
	return "СП 55.13330"
}
