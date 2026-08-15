# STAIR PLATFORM

**Document:** EDR-0022_CAD_Export.md

**ID:** EDR-0022

**Status:** APPROVED

**Author:** Project Team

**Date:** 2026-08-15

**Category:** Integrations

---

# 1. Purpose

Документ фиксирует CAD-экспорт (Phase E, E1) — выгрузку 3D-модели лестницы
в отраслевые форматы (DXF, STL, SVG) для передачи в САПР, станки с ЧПУ и
документацию. Экспорт строится на чистой стандартной библиотеке Go (DEV-0009);
сторонние CAD-библиотеки (go-dxf, go-stl и т.п.) недоступны офлайн.

Скоуп E1:

1. **Форматы** — `DXF` (R12 ASCII, 3DFACE), `STL` (ASCII), `SVG`
   (2D-проекция сетки).
2. **Источник данных** — полигональная сетка `kerngeo.Mesh` (ENG-GEO-0008),
   детерминированно пересчитываемая из сохранённой конфигурации проекта.
3. **API** — `GET /api/v1/projects/{id}/export/cad?format=dxf|stl|svg`.
4. **Качество** — unit-тесты writer'ов на эталонные треугольники; проверка
   инвариантов формата (заголовки секций, порядок вершин, корректные числа).

---

# 2. Related Artifacts

- ROADMAP-0011 (Phase E — Integrations, CAD)
- ENG-GEO-0008 (Mesh — производная величина, пересчёт из параметрической модели)
- EDR-0012 (Configuration Versioning — исходные данные для пересчёта)
- EDR-0013 (Audit Log — запись экспорта)
- API-0011 (Partner API: модели CAD-экспорта)
- ADR-0006 (Layered Architecture — writer'ы в infrastructure)
- DEV-0009 (Offline builds — только stdlib)

---

# 3. Model

## 3.1 Данные

Источник — сетка `kerngeo.Mesh`:

```go
type Mesh struct {
    Vertices  []Point3    // X, Y, Z — мм
    Triangles [][3]int    // индексы вершин
}
```

Сетка всегда пересчитывается детерминированно из конфигурации (инвариант
ENG-GEO-0008): CAD-экспорт принимает текущую сохранённую конфигурацию
проекта (EDR-0012), прогоняет конвейер `stair.Service.Calculate` и сериализует
полученный mesh. Это гарантирует точное совпадение CAD-модели с числами
проекта и исключает риск рассинхронизации с ра mid-loop конфигурациями.

## 3.2 Форматы

| Формат | MIME | Расширение | Описание |
|--------|------|------------|----------|
| `dxf`  | `application/dxf`  | `.dxf` | AutoCAD DXF R12 ASCII; каждая грань — `3DFACE` |
| `stl`  | `model/stl`       | `.stl` | ASCII STL; минимально валидный для слайсеров/ЧПУ |
| `svg`  | `image/svg+xml`   | `.svg` | 2D-проекция сетки (вид сверху, ортографическая) |

Writer'ы размещены в `internal/infrastructure/cad` — чистое преобразование
`io.Writer` ← mesh; без зависимостей от application/transport (ADR-0006).

## 3.3 Пакет `internal/infrastructure/cad`

```go
package cad

type Format string

const (
    DXF Format = "dxf"
    STL Format = "stl"
    SVG Format = "svg"
)

func ParseFormat(s string) (Format, error)

// WriteDXF сериализует сетку в ASCII DXF R12.
func WriteDXF(w io.Writer, m *kerngeo.Mesh) error

// WriteSTL сериализует сетку в ASCII STL.
func WriteSTL(w io.Writer, m *kerngeo.Mesh, name string) error

// WriteSVG сериализует ортографическую проекцию сетки (вид сверху) в SVG.
func WriteSVG(w io.Writer, m *kerngeo.Mesh) error

// Write выбирает writer по формату.
func Write(w io.Writer, m *kerngeo.Mesh, f Format) error
```

### DXF (R12 ASCII)

- Секции `HEADER` (минимальная, версия AC1009), `ENTITIES`.
- Каждая грань — `3DFACE` с 4 углами (последний повторяет третий — грань
  треугольная), координаты — в мм как есть.
- Имена сущностей групповым кодом `0`, координаты `10/20/30`,
  `11/21/31`, `12/22/32`, `13/23/33`.

### STL (ASCII)

- `solid <name>` / `facet normal nx ny nz` / `outer loop` / `vertex x y z` ×3
  / `endloop` / `endfacet` / `endsolid`.
- Нормаль — единичный вектор правого винта от порядка вершин
  (AB × AC, нормированный).

### SVG (вид сверху, XY-плоскость)

- Ортографическая проекция: `x = p.x`, `y = p.y`; `z` отбрасывается.
- Отрисовка как ломаных ребёр всех треугольников (стиль «wireframe»).
- `viewBox` вычисляется по bounding box проекции с отступом 4 мм;
  координата Y инвертируется (SVG-ось направлена вниз).
- Склейка точек с точностью 1e-6 (приведение float) для замкнутости контура.

## 3.4 Прикладной слой

`Application/project`: новый метод сервиса

```go
// ExportCAD возвращает сетку текущей конфигурации (пересчёт — детерминированный).
func (s *Service) ExportCAD(ctx context.Context, tenantID, userID, projectID string) (*kerngeo.Mesh, error)
```

- Требует членства (owner/editor/viewer) и права `project.read`.
- Загружает последнюю сохранённую конфигурацию (EDR-0012).
- Выполняет `s.calc.Calculate` (детерминированный конвейер), возвращает mesh.
- Нет конфигурации → `ErrNotFound`; сбой конвейера → ошибка.

## 3.5 Транспорт

`GET /api/v1/projects/{id}/export/cad?format=dxf|stl|svg` (auth, член проекта):

- `format` отсутствует/неизвестен → `400 invalid_input` (список допустимых).
- Нет проекта/членства → `403 forbidden`; нет конфигурации → `404 not_found`.
- Успех → `200` с `Content-Disposition: attachment; filename="<id>.dxf|stl|svg"`.
- Запись события аудита `project.export` (best-effort, EDR-0013).

---

# 4. Invariants

```
1. Экспорт детерминирован: одинаковые конфигурации дают одинаковый файл.
2. Writer'ы чисто функциональны: io.Writer ← mesh, без side-эффектов.
3. STL-нормали согласованы с порядком вершин (правая тройка).
4. DXF-SVG-инварианты проверены unit-тестами (корректные секции/теги).
5. CAD-экспорт доступен всем членам проекта (read), не только редакторам.
6. Только stdlib (DEV-0009); внешних CAD-зависимостей нет.
```

---

# 5. Schema

Без изменений схемы БД (ревизии конфигурации уже персистентны, EDR-0012).

---

# 6. API

| Method | Path | Auth | Result |
|--------|------|------|--------|
| GET | `/api/v1/projects/{id}/export/cad?format=dxf`  | auth+member | 200 `application/dxf` → `.dxf` |
| GET | `/api/v1/projects/{id}/export/cad?format=stl`  | auth+member | 200 `model/stl` → `.stl` |
| GET | `/api/v1/projects/{id}/export/cad?format=svg`  | auth+member | 200 `image/svg+xml` → `.svg` |
| GET | `/api/v1/projects/{id}/export/cad`             | auth+member | 400 invalid format |

---

# 7. Tests

1. `cad/dxf_test.go` — один треугольник: валидные секции, заголовок `AC1009`,
   ровно один `3DFACE` с четырьмя углами (последний = третий).
2. `cad/stl_test.go` — единичная нормаль, порядок вершин, замкнутость
   `solid`/`endsolid`.
3. `cad/svg_test.go` — валидный XML, тег `<polyline>`/`<path>`, viewBox из bbox.
4. `cad/cad_test.go` — `ParseFormat` (dxf/stl/svg/unknown), `Write` диспетчер
   (неизвестный формат → ошибка).
5. Пакет не зависит от HTTP/БД → tests быстрые и чистые.