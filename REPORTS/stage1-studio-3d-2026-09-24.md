# Этап 1 «студийный 3D» — отчёт (2026-09-24)

Ветка: `dev/swarm/s141-fixes` · не пушить без команды человека.
Коммиты: `98857a1`, `f250d6c`, `cfe18cc`, `cf055c9`, `56738e3`, `c9cf8a4`, `80fa41d`.

## Что сделано

| Шаг | Содержание | Файлы |
|---|---|---|
| 0 | Удалён временный probe-файл размера меша | — |
| 1 | Каталог материалов: 7 кодов (орех/ясень/сосна/кортен добавлены), цены-заглушки ₽/кг, инвариант «у каждого кода есть цена» (тест) | `internal/engine/manufacturing/material.go`, `internal/engine/pricing/pricing.go` |
| 2 | `Mesh.PartRanges` — роли деталей (tread/stringer/landing/railing_*) в preview-mesh | `internal/geometry/mesh.go`, `internal/engine/geometry/{mesh,engine}.go` |
| 3 | Студийный рендер: ACES Filmic, exposure 1.05, sRGB, PCF Soft shadows, IBL через PMREM + `RoomEnvironment` (фолбэк) и опциональный HDRI | `frontend/shared/src/viewer/GeometryViewer.tsx` |
| 4 | PBR-материалы по ролям: `materials.ts` (цвет/нормали/шероховатость/AO, sRGB против NoColorSpace, кэш, ленивая загрузка), стекло/металл ограждения | `frontend/shared/src/viewer/materials.ts` |
| 5 | Ассеты CC0: 7 материалов × (color/normal/roughness) с ambientCG (1K-JPG, 1024px) + HDRI Poly Haven `studio_small_08_1k`; Go-раздача `/static-assets/` | `hack/fetch_pbr.py`, `assets/pbr/SOURCES.md`, `internal/transport/http/static_assets.go` |
| 6 | «Спасти расчёт» — один клик применяет ближайший рабочий вариант для заблокированного конфига (основной сценарий — спираль) | `frontend-store/src/components/Constructor.tsx` |
| 7 | 7 материалов в UI с реальными фото-текстурами, авто-подстановка толщины в допуск материала, 5 пресетов готовых решений | `frontend/shared/src/config.ts`, `Constructor.tsx`, `App.css` |

## Гейты

- `go test -race -p 1 ./...` — **61/61 пакетов ok, 0 FAIL** (лог `/tmp/race-stage1.log`).
- `frontend`: tsc 0 ошибок, **432/432** тестов.
- `frontend-store`: tsc 0 ошибок, **2103/2103** теста.
- `oxlint` — без ошибок (только предсуществующие warning'и, ни один в новых файлах).
- Новые Go-тесты: `TestStaticAssetsHandler` (traversal/листинг/кэш/405), `TestPublicQuoteMeshHasPartRanges`, `TestEveryCatalogMaterialHasPrice`.

## Решения и компромиссы

- **Цены материалов — заглушки** (варинт Б пользователя): орех 220, ясень 60, сосна 25, кортен 180 ₽/кг. Реальные ставки выставим позже; инвариант «код ⇒ цена» защищает от забытых материалов.
- **Раскрой листовым** (6000×3000) остаётся приблизительным для дерева (это доски/фанера, не лист) — отдельная задача раскроя, помечена.
- **Толщина по допуску материала** считается в UI по `rulesFor('stepThicknessMM')`, а не по каталогу: у стали выпуск ступени 3–8 мм при каталожных 2–60 мм.
- **Бинарники ассетов не в git** (2.9 МБ): только манифест `SOURCES.md` с источником, лицензией CC0 и SHA-1; воспроизводит `hack/fetch_pbr.py`.
- **AO-карт нет** в 1K-JPG ambientCG — `aoMap` необязателен, код это переживает.

## Известные ограничения

- HDRI путь захардкожен дефолтом в `QuoteResult` (`/static-assets/hdri/…`); если Go-API отдаётся с другого домена, путь нужно вынести в конфиг фронта.
- Стеклянные перила — прозрачный MeshStandardMaterial с env-отражениями, без `transmission` (осознанно: дёшево на ~450 треугольниках).
- LOD не нужен: меш 27–31 KB (~800 вершин), измерено ранее.

## Следующий шаг

Этап 2 «конструктор»: интерактивное редактирование марша (перенос ступеней, смена направления подъёма) поверх уже готовых PartRanges и PBR-материалов.
