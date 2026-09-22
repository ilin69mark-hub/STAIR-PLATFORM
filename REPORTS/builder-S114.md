# S-114 (chore(ci)) — отчёт builder

**Вердикт:** выбран вариант (a) — guard-джоба без `if` превращает quirk-раны на push веток в зелёные no-op, а реальные release/deploy гейтятся по событию (`ref_type == 'tag' || workflow_dispatch`) с проверкой секретов уже внутри шагов через `jobs.<id>.env`-мост; 0-job failure-раны на ветках исключены, CD по тегам v* и вручную сохранён как раньше.

## Проблема и диагноз (подтверждён)

- Workflow `CD` создаёт **failure-раны с 0 джоб** на каждый push ветки (пример из карточки: run 35643279711, push dev/swarm/security-w5-w6, jobs=[], conclusion=failure), хотя триггер `push: tags v*` + `workflow_dispatch`.
- GitHub Actions действительно создаёт run на branch-push несмотря на tags-only фильтр (известный квирк) — в рабочем дереве подтверждено: файл как раз обрабатывает этот случай.
- Корень красного цвета: job-уровневый `if: ${{ secrets.AWS_ROLE_TO_ASSUME != '' }}` на строках release/deploy. **Секреты НЕ поддерживаются в `jobs.<id>.if`** (GitHub docs, таблица "Context availability", secrets отсутствует в этой строке) → условие всегда false → джобы скипаются -> run с 0 джоб -> conclusion=failure. Диагноз карточки подтверждён по итоговому состоянию файла: секреты перенесены в `jobs.<id>.env` (там доступны) и проверяются в `steps.<id>.if` через `env.*` (там env доступен).

## Выбранный вариант и обоснование

**Вариант (a)** — guard-джоба, которая ВСЕГДА запускается (без `if`) и завершается success с `::notice::`:
- `guard` (runs-on: ubuntu-latest, один step-Notice) — на quirk-run по push ветки джоба выполняется, release/deploy скипаются по событию -> run зелёный no-op вместо красного 0-job failure. Одновременно это явный notice в UI, что CD запускается только по тегам/вручную.
- `release`/`deploy` — `if: ${{ github.ref_type == 'tag' || github.event_name == 'workflow_dispatch' }}`, `environment: production` сохранён. Секреты проброшены в `jobs.<id>.env` (`AWS_ROLE_TO_ASSUME: ${{ secrets.AWS_ROLE_TO_ASSUME }}`) и каждый AWS-шаг получил `if: ${{ env.AWS_ROLE_TO_ASSUME != '' }}` + guard-step с notice при пустом секрете (режим "staging off", как и раньше — workflow не падает без секретов).
- Бонус-фикс: в `workflow_dispatch.inputs.version.description` было выражение `sha-${{ github.sha }}` — в описаниях inputs выражения не поддерживаются; заменено на литерал `sha-<commit sha>` (латентный конфиг-баг, мог ломать UI dispatch).
- **Вариант (b) отклонён**: удаление `push: tags` убило бы авто-CD по тегам (P3-14) даже после появления AWS-секретов; вариант (a) закрывает шум и сохраняет оба триггера с грацией при отсутствии секретов.

Поведение при наличии секретов идентично прежнему: tag v* / workflow_dispatch -> release (build+push 4 образов в ECR) -> deploy (helm upgrade ./deploy/helm/stair-platform, environment: production). При этом даже при заданных секретах push ветки больше не запускает release (раньше job-`if` по секрету НЕ фильтровал событие, и quirk-run на ветке с заданным секретом выполнил бы CD — латентный баг, закрыт попутно).

## Что изменено

- `.github/workflows/cd.yml` (+65/−16):
  - шапка-комментарий переписан под новое поведение (квирк GH, guard, мост секретов через jobs.env, список secrets/variables);
  - добавлена джоба `guard` (без `if` — всегда success);
  - `release`/`deploy`: `if` по `secrets` → по событию; `needs: guard` у release;
  - секреты перенесены в `jobs.<id>.env`, все AWS-шаги — с `if: env.AWS_ROLE_TO_ASSUME != ''`, guard-notice при пустом секрете (убран `continue-on-error: true` у skip-marker — больше не нужен);
  - фикс описания input.version; EOF newline.

## Проверки (DoD)

1. **YAML валиден**: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/cd.yml'))"` — парсится без ошибок; jobs: guard/release/deploy, on: push+workflow_dispatch. (pyyaml видит ключ `on` как boolean-ключ YAML 1.1 — известный квирк парсера, на GitHub (свой YAML-парсер) `on:` валиден; правка колон-в-имени шага из S5-3 учтена — двоеточий внутри имён/значений step-name нет.)
2. **Логика триггеров (статически)**:
   - push ветки (в т.ч. quirk-run): guard=success -> release/deploy `if` false -> skip -> **run зелёный no-op** (или не создаётся), 0-job failure исключён;
   - push тега v*: ref_type=tag -> release(+deploy) выполняются; без секретов — guard-notice + success (staging off), с секретами — полный release→deploy, environment: production;
   - workflow_dispatch: event_name=workflow_dispatch -> как по тегу; APP_VERSION = inputs.version || github.sha.
3. **Не запускал workflow** (нет секретов AWS; `gh workflow run` без dry-run не даёт статической проверки — по условию карточки ограничился статикой). Пути артефактов проверены: `deployments/{Dockerfile,admin.Dockerfile,store.Dockerfile}` и `deploy/helm/stair-platform` (Chart.yaml/templates/values.yaml) существуют.
4. Go-гейты не применимы (зона: `.github/workflows/`, изменений в Go-коде нет).

## Артефакты

- branch: `dev/swarm/ci-cd-noise` (от origin/main 19ef22b, up to date).
- commit: `chore(ci): S-114 — cd.yml больше не создаёт 0-job failure-раны на push веток` (не запушен — пуш/PR делает координатор).

## Блокеры

- Нет. Ожидание: пуш ветки координатором + PR; после мержа на main quirk-раны на ветках станут зелёными no-op (проверить на ближайшем push dev/swarm/*).
