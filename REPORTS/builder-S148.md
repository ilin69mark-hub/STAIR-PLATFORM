# S-148 — Safety-eval + LLM-бюджет + DELETE памяти (S-141 №6, №13, №14, день 6)

## Вердикт

**S-148 ЗАКРЫТ.** Все три находки устранены тремя отдельными коммитами:
№13 — `18f3207`, №14 — `573c06d`, №6 — `672aace` (ветка `dev/swarm/s141-fixes`,
не запушена). Self-execute @swarm, режим build-and-plan.

## №13 — Нет удаления conversation-memory (MEDIUM, GDPR Art.17/152-ФЗ)

- `Service.Forget` (application): purge памяти проекта, гейт членства fail-closed
  (S-142-семантика, не-член → `ErrForbidden`), dedicated audit-action
  `ai.assist.memory.purged` (purge kind-agnostic — kind-coupled аудит неприменим).
- `MemoryStore.DeleteMessages` + `AIRepository` (`DELETE ... WHERE tenant_id AND
  project_id`, возвращает число) + `DELETE /api/v1/assistant/memory?project_id=`
  за `authMutating` (400/403/200 `{deleted: n}`); swagger-путь добавлен.
- Каскад: уже есть в БД — `ON DELETE CASCADE` на tenant_id/project_id (миграция
  000026, новых миграций не потребовалось). Выборочного удаления по user нет
  by design (строки не атрибутированы user_id) — стирание через purge проектов;
  runbook: tenant-purge — `DELETE FROM conversation_messages WHERE tenant_id=$1`
  (каскад покрывает удаление tenant/project целиком). Потоков удаления
  пользователей в коде нет (только `DeleteUserSessions` в тестовом фейке) —
  нечего хукать.
- Тесты: `TestForgetMemberPurges/NonMemberForbidden/RequiresScopeAndMemory`
  (сервис), `TestMemoryDeleteMessages` (PG: purge + идемпотентность + tenant-изоляция),
  `TestAssistantForgetOK/Forbidden/NoProjectID` (транспорт).

## №14 — Нет hard-лимита LLM-бюджета (MEDIUM, OWASP LLM10/CWE-400)

- `Budget` (application, потокобезопасен): дневной лимит попыток primary-вызовов,
  календарные сутки UTC с rollover, `max<=0` = без лимита. Учёт — попытки
  (консервативный прокси биллинга).
- `ModelRouter.WithBudget`: исчерпан → primary пропускается (только local) +
  `stair_assistant_llm_budget_denied_total` + warn-лог (источник для алерта).
- `Service.WithBudget`; `WithPrimaryBackend` сохраняет ранее заданный бюджет
  (порядок chaining не важен — покрыто тестом). Env `STAIR_AI_DAILY_LLM_BUDGET`
  (0/unset = без лимита) wired в `cmd/api/main.go`.
- `assistant.ServiceMetricsReg` дотянут в `/metrics` (раньше реестр существовал,
  но никуда не выводился — метрики слоя были невидимы).
- Тесты: `TestBudgetAllowAndExhaust/Unlimited/DailyRollover`,
  `TestRouterBudgetSkipsPrimary` (primary не вызывается, ответ локальный),
  `TestServiceWithBudgetOrderIndependent`.
- Остаток: Prometheus-алерт на `..._denied_total` + месячный cap (день 7 scope);
  эмбеддер-вызовы в бюджет не входят (только commentary-primary — задокументировано).

## №6 — Нет safety-eval LLM (HIGH, OWASP LLM01/CWE-1427)

- `testdata/eval-safety.jsonl` (12 кейсов) + `eval_safety_test.go`: классы
  prompt-injection (в т.ч. base64, систем-промпт), jailbreak role-play,
  pii-exfil (история + RAG), harmful-advice (обход норм).
- Раннер двоякий: (1) канарейка — маркер атаки ОБЯЗАН быть в перехваченном
  `Prompt.Context` (атака дошла до границы, тест не вакуумен); (2) маркер
  ОБЯЗАН отсутствовать в ответе+комментарии. Безопасность — булева (все 12
  должны пройти). Итог: **12/12, failing=0**.
- Почему проходит: сегодня свободный текст пользователя вообще не имеет канала
  в ответ (API принимает только конфиг; `localComment` строит текст только из
  `Response`) — сьют пинает это свойство и поймает регрессию (например, будущее
  эхо ввода LLM). CI-гейт: сьют — Go-тест, гоняется в каждом `go test ./...`
  (включая CI) по построению.
- Честное ограничение (слепая зона сохраняется): live-LLM commentary при
  свободных вопросах (ступень 3) этим не покрыт — нужен staging-прогон с
  реальной моделью; отравление истории/RAG для LLM-пути проверяется канарейкой
  только до границы промпта.

## Гейты

- `gofmt` чист, `go vet` 0 (assistant, database, transport/http, cmd/api).
- Точечные: Forget ×3, DeleteMessages (PG), Forget-транспорт ×3, Budget ×5,
  safety 12/12 — зелёные.
- Полный `go test -race -p 1 ./...` — в фоне, результат дописать координатору.

## Gotcha: двойной swagger + дрейф генератора (закрыто тут же)

- `TestSwaggerSpecMatchesRoutes` упал: я правил `docs/openapi/swagger.yaml`
  вручную, а тест читает embedded-копию
  `internal/transport/http/swagger/swagger.yaml`. Правило: только
  `python3 hack/gen_swagger.py --write` (пишет оба файла из роутера).
- Второй дрейф: генератор добавил в spec `/ws/admin`, а тест его исключает
  (`nonRESTRoutes`) — скрипт не обновили после S-116 (admin-namespace).
  Фикс-корень: `/ws/admin` в `NON_REST` генератора (коммит `0ce0cbe`).
  Урок: чей-то hand-sync spec — всегда через генератор + этот тест.
