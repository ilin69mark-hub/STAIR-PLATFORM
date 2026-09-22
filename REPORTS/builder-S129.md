# S-129: Ops-процедуры — incident/rollback/on-call

**Вердикт:** DONE — новый док + 3 бокса чеклиста закрыты, запушено в `dev/swarm/ws-s115-s116`.

## Что сделано
- `docs/17_INFRASTRUCTURE/26_INCIDENT_RESPONSE.md` (новый, INFRA-0026): SEV-1/2/3 с реакцией/митигацией/postmortem-сроками; роли IC/Comms/On-call; flow detect→postmortem; rollback runbook (helm rollback, migrate -down с дампом сначала, фронты redeploy sha-тега, пост-проверки); on-call handoff + тишина-пейджера проверка.
- `docs/19_ROADMAP/18_PRODUCTION_LAUNCH_CHECKLIST.md` — 3 бокса Operations отмечены [x] со ссылкой на S-129; `Critical contacts` оставлен человеку (нужны реальные имена/телефоны).

## Гейты (docs-зона, код не тронут)
- Команды в runbook сверены с репо: `cmd/migrate -down` существует, up/down-пары (49 файлов), Helm-релиз `stair`, immutable sha-теги (S2-3), backup/restore скрипты (S-128).

## Счётчик CI
Задача 16/20 после PR #51 (следующий CI-прогон после 20-й).
