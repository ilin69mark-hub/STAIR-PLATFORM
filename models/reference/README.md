# Эталон FreeCAD — `stair-comb-railings.FCStd`

Источник истины о размерах деталей (S-160, S-196). Перенесён в git из
`../freecad-out/` (вне репозитория, в CI недоступен), чтобы гейт
`parts_catalog_test.go` сверялся с файлом из репозитория, а не с соседним
каталогом на чьём-то диске.

- **Файл:** `stair-comb-railings.FCStd` (285 КБ), sha256
  `9de0dcd40774ac35f0f91122a4186aa7bc4935dd2fc1d69abd7c2e73f8e70eea`.
- **Что внутри:** комбинированная лестница; каждая деталь — отдельный объект
  `Part::Feature`. Именование: вид + сторона/индекс, например `Stringer_A`,
  `Web_A_01`, `Heel_A_01`, `Anchor_A`, `FirstWeb_A`, `TopAnchor_A`,
  `Tread_01`…`Tread_16`, `Riser_01`…`Riser_16`, `Rail_Post_01`…,
  `Rail_Portun` (поручень), `Rail_Rigel_1_01`… / `Rail_Rigel_2_01`… (оба ряда —
  одна позиция), `Rail_Tip_01`…, `Rail_Lid_01`…, `Rail_Cap_Start/End`.
- **Как перегенерировать эталонный JSON:** открыть модель во FreeCAD и
  выполнить `../freecad-out/export_catalog.py` (он же пишет
  `reference_parts.json`: bbox + volume 14 позиций, допуск 0,3 мм / 3 %).
  Долгосрочно скрипт переедет в `hack/gen_part_manifest.py` (S-198).
- **Известная особенность (не править модель):** в эталоне фланец сплавлен
  со стойкой, поэтому bbox стойки — Ø86, а труба на деле Ø42,4×1,5.
  Наружные диаметры труб берутся из констант `fix_railings.py`
  (`POST-001`/`RAIL-P` — 42,4×1,5; `RIGL-001` — 12×1), а не из bbox.
  Это задокументировано и в самом `export_catalog.py` (`TUBE_SPEC`).
