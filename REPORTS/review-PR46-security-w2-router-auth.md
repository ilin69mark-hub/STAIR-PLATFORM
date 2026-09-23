# Review: PR #46 — security W2: S-106 + S-108 + S-109 router/auth hardening

Branch: `dev/swarm/security-w2-router-auth` → `main`
Commit: `2c5737f`

## Verdict: ✅ APPROVED

> self-approve forbidden → вердикт оформлен via comment + REPORTS, merge —
> по явной команде владельца (протокол репо). Task-dispatch субагентов ⚙️
> BLOCKED_INFRA (`Model not found: anthropic/claude-sonnet-4-5`) → рой
> выполнил W2 self-execute (протокол §9.1).

## Чек-лист

| Проверка | Результат |
|---|---|
| `gofmt -l` (изменённые файлы) | ✅ чисто |
| `go vet ./...` | ✅ clean |
| `go build ./...` | ✅ clean |
| `go test ./...` | ✅ 58 пакетов ok, 0 fail |
| `./scripts/coverage-check.sh` | ✅ 87.0% ≥ 85% |
| `golangci-lint` (затронутые пакеты) | ✅ новых нет (6 предсуществующих вне диффа) |
| Секреты в диффе | ✅ нет |
| Фронт-регресс (CSRF на новых роутах) | ✅ роуты из UI не вызываются (`rg` по frontend/frontend-store — 0) |

## Что проверено

### S-106 (`internal/transport/http/sso.go`)
`isSafeRedirect` теперь: (a) требует ведущий `/`; (b) отклоняет `\` в raw;
(c) отклоняет `%5c` в любом регистре; (d) отклоняет `\` в `parsed.Path`
(percent-декод `%5C`). Порядок проверок корректен, `url.Parse` остаётся
последним рубежом (`IsAbs`, `//`). Over-rejection (`/x?q=a%5Cb`) — приемлемо:
redirect-цель с backslash подозрительна, фолбэк на `/` безопасен.

### S-108 (`internal/transport/http/router.go`)
4 роута: `DELETE /integrations/endpoints/{id}`, `POST /projects/{id}/quote-send`,
`/crm-sync`, `/order-send` → `authProtected → authMutating`
(`requireAuth` ∘ `requireCSRF`). Порядок мидлварей в `authMutating` сохранён
(CSRF внутренний — auth наружу), как у остальных mutating-роутов.
Регресс-тест `TestMutatingIntegrationRoutesRequireCSRF` (4 сабтеста): сессия без
CSRF → 403. Позитивные тесты (`TestQuoteSend` и др.) используют `authedRequest`
(ставит csrf cookie+header) — не сломаны.

### S-109 (`router.go`, `middleware_auth.go`, `cmd/api/main.go`, `database/auth_repo.go`)
- `Config.SsoRateLimit/SsoRateWindow` + дефолт 20/min; fallback-блок в
  `NewRouter` в стиле остальных лимитеров; env `STAIR_SSO_RATE_LIMIT`.
- `ssoLimiter` на 3 публичных SSO-роута (`/auth/sso`, `/callback`, `/config`).
- `CreateSsoState`: prune истёкших + cap `maxSsoStates=10000` (удаление самых
  старых по `created_at`). Нюанс: два `Exec` до INSERT — приемлемо при
  rate-limit; при росте нагрузки стоит вынести в транзакцию/индекс по `created_at`.
- `TestRateLimitSso` — 2×200 → 3-й 429. Учтён response-cache (GET `/config`
  кэшируется) — уникальный query на запрос, чтобы дойти до лимитера.

## Заметки
- `POST /stairs:calculate|validate|optimize`, `POST /assistant/{kind}` остаются
  `authProtected` — stateless-вычисления, аудитом S-103 не флагались (вне scope W2).
- `maxSsoStates` — пакетная константа; при желании вынести в Config (не блокер).

**Вердикт:** APPROVED — 3 фикса корректны, гейты зелёные, регресс-тесты добавлены; merge по команде владельца.
**Нюансы:** Task-dispatch субагентов ⚙️ BLOCKED_INFRA (неверный `model` в `.opencode/agent/*.md`); W2 выполнен self-execute.
**Рекомендации:** починить `model` агентов (валидный id) для восстановления диспатча; W3 — S-107 (cache/no-store).
