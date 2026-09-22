# S-123b: SHA-pinning GHA-экшенов (pre-CI)

**Вердикт:** DONE — 53 `uses:` в 3 workflow запинены на SHA с версией в комменте, YAML валиден, запушено в `dev/swarm/ws-s115-s116`.

## Что сделано
- `.github/workflows/ci.yml` (40), `cd.yml` (10), `nightly-load.yml` (3): все `uses: X@tag` → `uses: X@<40-hex-sha> # tag (S-123b)`. Ноль непокрытых (проверено grep).
- SHA резолвнуты через GitHub API tag-ref (annotated → deref до коммита); checkout@v4 пин совпал с общеизвестным `11bd719...` — метод верный.

## Гейты
- `yaml.safe_load` всех workflow — ok; дифф чисто механический (53+/53-, логики ноль).
- Полный локальный: 58/58 race+БД на свежей БД (гейт перед PR).
- Полная валидация — CI-прогон PR (единственное место).

## Счётчик CI
CI-гейт 20/20: этот коммит едет в PR.
