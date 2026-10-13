package engineering

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"stairplatform/models"
)

// Сверка каталога parts.go с вшитым манифестом FreeCAD (S-199).
//
// Манифест — это замеры реальной модели, каталог — их перенос в код.
// Любое расхождение означает одно из двух: либо код врёт про деталь,
// либо манифест устарел. Оба случая — падение, а не скип (см. S-194).

// manifestOrFail — манифест обязан разбираться. Нераскрывшийся манифест —
// это испорченный репозиторий, а не «нет данных»: пропускать нельзя.
func manifestOrFail(t *testing.T) models.Manifest {
	t.Helper()
	m, err := models.Load()
	if err != nil {
		t.Fatalf("вшитый манифест не разобрался: %v", err)
	}
	if len(m.Parts) != 14 {
		t.Fatalf("в манифесте %d деталей, ожидалось 14 — манифест или тест устарели", len(m.Parts))
	}
	return m
}

// TestManifestCoversCatalog — каждая деталь манифеста есть в каталоге
// с теми же группой, формой, материалом и именем.
//
// Группа/форма/материал определяют судьбу детали (раскрой, закупка, цена):
// расхождение здесь — это деталь, которая режется не из того или
// покупается не та. Ловит «поменял в одном месте, забыл в другом».
func TestManifestCoversCatalog(t *testing.T) {
	m := manifestOrFail(t)
	for _, mp := range m.Parts {
		spec, ok := PartByID(mp.ID)
		if !ok {
			t.Errorf("%s: есть в манифесте FreeCAD, но отсутствует в каталоге parts.go", mp.ID)
			continue
		}
		if string(spec.Group) != mp.Group {
			t.Errorf("%s: группа каталог %q, манифест %q", mp.ID, spec.Group, mp.Group)
		}
		if string(spec.Form) != mp.Form {
			t.Errorf("%s: форма каталог %q, манифест %q", mp.ID, spec.Form, mp.Form)
		}
		if string(spec.Material) != mp.Material {
			t.Errorf("%s: материал каталог %q, манифест %q", mp.ID, spec.Material, mp.Material)
		}
		if spec.Name != mp.Name {
			t.Errorf("%s: имя каталог %q, манифест %q", mp.ID, spec.Name, mp.Name)
		}
	}
}

// fusedInReference — позиции каталога, которых НЕТ в эталонной модели
// как отдельных объектов. Фланец и накидная часть сплавлены со стойкой
// в stair-comb-railings.FCStd (см. models/reference/README.md), поэтому
// их отсутствие в манифесте — известное свойство эталона, а не
// «деталь пропала». Список закрытый: новая запись сюда — только
// с причиной в комментарии.
var fusedInReference = map[string]string{
	"FLNG-001": "фланец сплавлен со стойкой POST-001 в эталоне",
	"SLIP-001": "накидная часть сплавлена со стойкой POST-001 в эталоне",
}

// TestCatalogCoveredByManifest — каждая позиция каталога есть в манифесте.
//
// Ловит «деталь пропала»: позицию добавили в код, а в модели её нет
// (или наоборот — тогда падает соседний тест). Исключения — только
// fusedInReference с причиной, молча пропускать нельзя.
func TestCatalogCoveredByManifest(t *testing.T) {
	m := manifestOrFail(t)
	for _, spec := range AllParts() {
		if reason, ok := fusedInReference[spec.ID]; ok {
			t.Logf("%s: отсутствует в манифесте осознанно — %s", spec.ID, reason)
			continue
		}
		if _, ok := m.ByID(spec.ID); !ok {
			t.Errorf("%s (%s): есть в каталоге, но отсутствует в манифесте FreeCAD", spec.ID, spec.Name)
		}
	}
}

// TestManifestVolumesSane — объёмы манифеста физически возможны (S-195).
//
// Поле volume раньше разбиралось, но ничем не сверялось: мёртвые данные.
// Независимого источника объёмов в Go нет (движок строит детали под
// конкретный проект, а эталон — конкретную 16-ступенчатую лестницу;
// считать «независимый» объём из тех же габаритов — тавтология).
// Поэтому здесь проверяются инварианты, верные для любой геометрии:
// объём положителен и не превышает объём габаритного ящика. Ловит
// порчу файла: лишний ноль, перепутанные поля, обрезанный JSON.
func TestManifestVolumesSane(t *testing.T) {
	m := manifestOrFail(t)
	boxVol := func(b models.Box) float64 { return b.L * b.W * b.H }
	for _, p := range m.Parts {
		if p.Volume <= 0 {
			t.Errorf("%s: объём %v — неположительный", p.ID, p.Volume)
		}
		if bv := boxVol(p.BBox); p.Volume > bv {
			t.Errorf("%s: объём %.1f больше габарита %.1f — так не бывает",
				p.ID, p.Volume, bv)
		}
		for _, s := range p.Solids {
			if s.Volume <= 0 {
				t.Errorf("%s/%s: объём тела %v — неположительный", p.ID, s.Object, s.Volume)
			}
			if bv := boxVol(s.BBox); s.Volume > bv {
				t.Errorf("%s/%s: объём тела %.1f больше его габарита %.1f",
					p.ID, s.Object, s.Volume, bv)
			}
			for i, sub := range s.SubSolids {
				if sub.Volume <= 0 {
					t.Errorf("%s/%s: субтело %d неположительное", p.ID, s.Object, i)
				}
				if bv := boxVol(sub.BBox); sub.Volume > bv {
					t.Errorf("%s/%s: субтело %d (%.1f) больше своего габарита (%.1f)",
						p.ID, s.Object, i, sub.Volume, bv)
				}
			}
		}
	}
}

// legacyVolumes — объёмы, снятые 2026-10-13 легаси-экспортом
// (../freecad-out/export_catalog.py → reference_parts.json) и сверенные
// с выходом нового генератора: расхождений ноль (S-198).
//
// Зачем эта копия: она замораживает проверенное состояние. Если генератор
// сломается (не тот представитель, не то округление, не та группировка),
// тест упадёт, хотя --check останется зелёным (детерминизм ≠ правильность).
// Обновлять — только вместе с осознанным изменением модели, с записью
// в карточке S-200, почему объёмы изменились. Допуск — на уровне округления
// (±0,05), а не 3 %: это сверка копий, а не замеров; 3 % здесь были бы
// враньём про строгость.
var legacyVolumes = map[string]float64{
	"COS-001":  8411567.0,
	"WEB-001":  31257.5,
	"HEEL-001": 44743.4,
	"ANCR-001": 44914.8,
	"FWEB-001": 17777.5,
	"TANC-001": 29114.8,
	"TRED-001": 9340685.8,
	"RISR-001": 4860000.0,
	"RAIL-P":   994683.9,
	"POST-001": 200058.0,
	"RIGL-001": 52055.2,
	"TIP-001":  40789.5,
	"DECO-001": 6071.0,
	"ENDP-001": 21209.1,
}

// TestManifestVolumesVsLegacy — манифест совпадает с проверенным состоянием.
func TestManifestVolumesVsLegacy(t *testing.T) {
	m := manifestOrFail(t)
	if len(legacyVolumes) != len(m.Parts) {
		t.Fatalf("золотых объёмов %d, деталей в манифесте %d — обновите карту вместе с манифестом",
			len(legacyVolumes), len(m.Parts))
	}
	for _, p := range m.Parts {
		want, ok := legacyVolumes[p.ID]
		if !ok {
			t.Errorf("%s: нет золотого объёма — новая деталь без проверки генератора", p.ID)
			continue
		}
		if math.Abs(p.Volume-want) > 0.05 {
			t.Errorf("%s: объём манифест %.1f, проверенный %.1f — генератор или модель изменились",
				p.ID, p.Volume, want)
		}
	}
}

// TestManifestModelFresh — sha256 модели совпадает с записанным в манифесте.
//
// Ловит «модель поменяли, манифест забыли»: главный дрейф пары
// «FCStd ↔ JSON». Путь — от файла теста через runtime.Caller, фиксированный
// относительный путь, а не поиск по дереву: детерминировано в любом
// окружении. Отсутствие файла — падение, а не скип.
func TestManifestModelFresh(t *testing.T) {
	m := manifestOrFail(t)
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller не отдал путь теста")
	}
	// internal/domain/engineering → корень репозитория.
	repo := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	modelPath := filepath.Join(repo, "models", m.SourceModel)
	//nolint:gosec // G304: путь собран из вшитого манифеста и положения
	// теста — ввод пользователя сюда не попадает.
	buf, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatalf("модель %s не читается: %v — манифест ссылается в пустоту", modelPath, err)
	}
	sum := sha256.Sum256(buf)
	if got := hex.EncodeToString(sum[:]); got != m.SourceSHA256 {
		t.Errorf("модель изменилась без регенерации манифеста:\n файл: %s\n манифест: %s",
			got, m.SourceSHA256)
	}
}
