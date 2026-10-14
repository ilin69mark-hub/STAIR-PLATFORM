package stair

import (
	"context"
	"testing"

	"stairplatform/internal/domain/engineering"
)

// TestPurposeSelectsNormProfile — назначение здания меняет профиль
// проверки (S-236): ограждение 850 мм в заказе на одноквартирный дом
// блокирует расчёт, потому что профиль жилого здания берёт ГОСТ 25772
// как блокирующее правило (S-239).
//
// Проверяем на сквозном пути (сервис → профиль → валидация), потому что
// дефолт живёт именно там: пустое назначение = дом.
func TestPurposeSelectsNormProfile(t *testing.T) {
	cfg := referenceConfig()
	cfg.RailingHeight = mustLengthHelper(850)

	// Явное «общественное здание» — тот же дефолт по нормам, но профиль
	// другой: важно, что назначение доезжает и не теряется по дороге.
	pub := cfg
	pub.Purpose = engineering.PurposePublic
	res, err := NewService().Validate(context.Background(), pub, Options{skipVariations: true})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !res.Validation.Blocking {
		t.Fatal("ограждение 850 мм должно блокировать в профиле по назначению (S-239)")
	}
	found := false
	for _, i := range res.Validation.Issues {
		if string(i.Code) == "SAF-RAILING-HEIGHT" {
			found = true
			if i.Source == "" {
				t.Error("у нарушения нет источника — S-235 потерялся по дороге")
			}
		}
	}
	if !found {
		t.Error("в нарушениях нет SAF-RAILING-HEIGHT")
	}
}
