package manufacturing

import (
	"testing"

	"stairplatform/internal/domain/engineering"
)

// TestPurchasedPartsComeFromCatalog — производство строит ограждение из
// engineering.PurchasedParts(), а не из собственных констант (S-180).
//
// Проверяем не «значения совпадают» (тогда тест повторил бы константы), а
// признак реальной связи: если позиция каталога потеряет размеры, ограждение
// молча соберётся из нулей — и тест это поймает.
func TestPurchasedPartsComeFromCatalog(t *testing.T) {
	for _, id := range []string{"POST-001", "RIGL-001", "TIP-001", "FLNG-001", "SLIP-001", "DECO-001", "ENDP-001"} {
		t.Run(id, func(t *testing.T) {
			spec := purchasedSpec(id)
			if spec.Form == engineering.FormTube {
				if spec.TubeOD <= 0 || spec.TubeWall <= 0 || spec.Len <= 0 {
					t.Fatalf("труба %s без геометрии: OD=%v wall=%v len=%v",
						id, spec.TubeOD, spec.TubeWall, spec.Len)
				}
				return
			}
			if spec.DimL <= 0 || spec.DimW <= 0 || spec.DimH <= 0 {
				t.Fatalf("фитинг %s без габаритов: L=%v W=%v H=%v",
					id, spec.DimL, spec.DimW, spec.DimH)
			}
		})
	}
}

// TestProductionDimsAreNotZero — размеры, которыми реально строится
// ограждение, не нулевые. Пустой lookup в каталоге дал бы деталь нулевой
// толщины и молчаливую ошибку геометрии.
func TestProductionDimsAreNotZero(t *testing.T) {
	checks := map[string]float64{
		"postOD": postOD, "postWall": postWall,
		"rigelOD": rigelOD, "rigelWall": rigelWall, "balusterLen": balusterLen,
		"tipSizeL": tipSizeL, "tipSizeW": tipSizeW, "tipSizeH": tipSizeH,
		"flangeOD": flangeOD, "flangeH": flangeH,
		"slipOD": slipOD, "decoOD": decoOD, "decoH": decoH, "endCapOD": endCapOD,
	}
	for name, v := range checks {
		if v <= 0 {
			t.Errorf("%s = %v — каталог не отдал размер", name, v)
		}
	}
}
