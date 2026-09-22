# builder — S-113-residual: disabled-аккаунт неотличим на login

**Задача:** закрыть остаточный нюанс из `REPORTS/builder-S113-fix.md` — ветка
«disabled user» в `Login` возвращала `ErrUserDisabled` без bcrypt: состояние
раскрывалось явным кодом ошибки (403 `user_disabled`), плюс тайминг-канал
(без bcrypt отвечала быстро).

**Выполнено:** координатором (self-execute).

---

## Изменения

### `internal/application/auth/service.go` — Login
Порядок веток изменён: **bcrypt-сравнение выполняется ДО проверки статуса**:

```
было:  GetUserByEmail → status check (ErrUserDisabled, без bcrypt) → bcrypt
стало: GetUserByEmail → bcrypt → status check (ErrInvalidCreds)
```

- Disabled-аккаунт с **верным** паролем → `ErrInvalidCreds` (состояние не
  раскрывается; аудит-журнал по-прежнему пишет «user disabled»).
- Disabled-аккаунт с **неверным** паролем → `ErrInvalidCreds` (как раньше).
- Тайминг: disabled-ветка теперь выполняет bcrypt с реальным хешем (тот же
  cost) → неотличима от «неверный пароль» и «неизвестный email».

### `internal/transport/http/auth.go` — handleLogin
Убран мёртвый кейс `ErrUserDisabled` → 403 `user_disabled` (Login его больше
не возвращает) + комментарий. Ответ для всех неуспешных логинов — единый
401 `invalid_credentials`.

### `Authenticate` — НЕ тронут (корректно)
Валидная сессия + disabled-аккаунт → `ErrUserDisabled`: пользователь уже
доказал владение сессией, состояние можно сообщить (не вектор энумерации).

## Тесты

- `TestDisabledUserCannotLogin` — переписан: ожидает `ErrInvalidCreds` и с
  верным, и с неверным паролем (состояние не раскрывается).
- `TestAuthenticateDisabledUser` — НОВЫЙ: фиксирует, что аутентифицированный
  путь по-прежнему возвращает `ErrUserDisabled`.
- OpenAPI: `/auth/login` не документировал 403 — правок не требуется.

## Гейты (локально)

| Гейт | Результат |
|---|---|
| gofmt -l | пусто |
| go vet ./... | 0 |
| go build ./... | ok |
| golangci-lint run | 0 issues |
| auth-тесты (Login/Authenticate/Disabled) | 15/15 PASS |
| **Полный прогон** `go test -race -p 1 ./...` + БД | **58/58 ok, 0 FAIL** |

Примечание: первый прогон дал 5 FAIL в analytics_repo_test — «Dirty database
version 17» (грязное состояние тестовой БД, не связано с правками); после
пересоздания контейнера `stair-test-pg` — 58/58.

## Итог

Все три тайминг/энумерационных канала на login закрыты:
1. неизвестный email — dummy-bcrypt (S-113-fix);
2. disabled-аккаунт — bcrypt до статуса + единый `ErrInvalidCreds`;
3. неверный пароль — bcrypt (как было).

**Вердикт:** disabled-аккаунт неотличим от неверного пароля/неизвестного
email на login (тайминг и код ошибки); состояние сообщается только
аутентифицированным (Authenticate) и в аудит-журнале. 58/58, lint 0.

**Длительность:** ~20 мин (self-execute).