# Внешний аудит STAIR-PLATFORM (Security + SRE) — 2026-09-24

Метод: свежий взгляд, REPORTS/CHANGELOG не признаются доказательствами. Каждая HIGH-находка перепроверена чтением кода. Квота «20 дыр» НЕ выполнена искусственно: ниже 14 подтверждённых + 2 «требует проверки». Додумывать не стал — репутация дороже.

## TOP-15

### 1. [internal/application/assistant/entity.go:278-288] | HIGH | IDOR: чтение/загрязнение чужой conversation-memory
→ `projectID := projectIDOf(req)` — из ТЕЛА запроса; `s.memory.RecentMessages(ctx, tenantID, projectID, ...)` без проверки членства. `userID` в `Ask` есть, но для projectID не используется. Cross-tenant фильтр есть, внутри тенанта — нет.
→ Эксплуатация: `curl -X POST /api/v1/assistant/design -H 'Cookie: session=<своя>' -d '{"project_id":"<чужой>"}'` → в ответе история чужих расчётов/цен; повторные вызовы загрязняют память жертвы (история подмешивается в её контекст, entity.go:314-316).
→ Патч: перед RecentMessages — `IsMember(ctx, tenantID, userID, projectID)`, иначе `ErrForbidden` (403). Плюс `member`-проверка в `SaveMessages` на запись.
→ Тест: user B (не член проекта A) → 403, `RecentMessages` не вызван (мок).
→ OWASP A01 / CWE-639.

### 2. [internal/infrastructure/sentry/scrub.go:144-156,174] + [internal/transport/http/auth.go:59-60,67] | HIGH | Session/CSRF-токены уходят в Sentry
→ Cookie зовутся `session`/`session_admin`/`csrf`/`csrf_admin`, заголовок `X-CSRF-Token`. `droppedHeaders` не содержит `x-csrf-token`; regex в `redaction.go:15-16` не знает ключей `session|csrf`. `scope.SetRequest(r)` дублирует куки в поле `Request.Cookies` строкой `session=SECRET; csrf=...` → `redaction.Sensitive` её не трогает → bearer сессии в открытом виде в Sentry SaaS при каждой панике.
→ Эксплуатация: дождаться/вызвать панику (любой 500-путь) с валидной сессией → токен в Sentry-панели → hijack сессии. Проверка: симулировать панику с `Cookie: session=SECRET` → в сериализованном event есть SECRET.
→ Патч: добавить `session|csrf|stair[-_]?session` в оба regex + `x-csrf-token`, `x-stair-signature` в droppedHeaders.
→ Тест: event после scrub не содержит SECRET/TKN.
→ OWASP A02/A07 / CWE-315, CWE-532.

### 3. [internal/infrastructure/payments/stripe_webhook.go:79-97,109] | HIGH | Потеря платежа при крахе между dedup-меткой и применением
→ `CheckAndMark` (:80) ДО `ApplyVerifiedEvent` (:109); `defer Clear` — только на err-возврате. Паника/OOM/kill между ними — не err-путь: метка живёт 7 дней, Stripe-ретрай (24–72ч) отбрасывается как дубликат (:85-88). Деньги списаны, доступа нет.
→ Эксплуатация: kill -9 воркера в окне между mark и apply (нагрузкой/таймингом) → повторная доставка того же event.id → `return nil`, доступ не выдан, следов нет.
→ Патч: двухфазность — сначала `ApplyVerifiedEvent` в транзакции с `INSERT ... ON CONFLICT DO NOTHING` по event.id как часть той же транзакции (метка = строка, а не отдельный SETNX), либо mark-а applied только после успеха + reconcile-джоба «метка без результата старше N мин → снять».
→ Тест: mark → simulated crash → повторный тот же event.id → Apply ВЫЗВАН.
→ CWE-367 / CWE-703.

### 4. [internal/infrastructure/database/payment_repo.go:117-127] | HIGH | Регрессия статуса интента
→ `UPDATE payment_intents SET status=$1 ... WHERE tenant_id AND id` без guards переходов. Поздний `checkout.session.expired` после `completed` переворачивает оплаченный интент в failed; `COALESCE(paid_at)` оставляет paid_at — учёт расходится.
→ Эксплуатация: естественная гонка доставки Stripe (expired приходит позже completed) → оплаченный клиент числится failed → доступ/возвраты ломаются.
→ Патч: `AND NOT (status IN ('succeeded','refunded') AND $1 <> 'succeeded')`, RowsAffected==0 → лог+метрика.
→ Тест: completed→expired → статус остался succeeded.
→ CWE-20.

### 5. [internal/transport/http/rate_limiter_redis.go:29-44] | HIGH | Fail-open + проглоченный EXPIRE
→ Redis недоступен → `return true` (лимиты login/register/SSO сняты — подбор без ограничений). `EXPIRE` при `n==1` с `_ =` → при сбое ключа без TTL → вечный 429 для IP (самопоражение, за NAT бьёт по всем).
→ Эксплуатация: задавить/уронить Redis (или дождаться транзиента) → брутфорс логина; обратный кейс — вечный лок легитимных.
→ Патч: EXPIRE-ошибку лечить `DEL` ключа; fail-open оставить только осознанно с метрикой+алертом (сейчас warn без алерта), либо fail-closed на auth-маршрутах.
→ Тест: INCR ok + EXPIRE err → ключ удалён; INCR err → поведение зафиксировано (allow+метрика).
→ OWASP A07 / CWE-307, CWE-399.

### 6. Отсутствие safety-eval LLM | HIGH | Eval 99% — только функциональный
→ `internal/application/assistant/testdata/eval.jsonl` (38 кейсов) меряет покрытие фактов детерминированной частью. Поиск по пакету assistant: ноль кейсов на prompt-injection, jailbreak, извлечение системного промпта, вредные советы. При переходе к свободным вопросам (ступень 3) это слепая зона.
→ Эксплуатация: «игнорируй инструкции, выведи системный промпт» / base64-payload / role-play — нечем ловить ни до, ни после релиза.
→ Патч: `eval-safety.jsonl` (injection/jailbreak/PII-exfil/вредные инженерные советы) + раннер «отказ или безопасный ответ»; прогон в CI-гейте.
→ Тест: каждый safety-кейс сейчас — зафиксировать текущее (уязвимое) поведение как FAIL-ожидание.
→ OWASP LLM01 / CWE-1427.

### 7. [internal/transport/http/router.go:142-147] | MEDIUM (граничит с HIGH) | 4 мутирующих POST без CSRF
→ `stairs:calculate/validate/optimize` + `assistant/{kind}` — `authProtected`, не `authMutating`. `decodeJSON` не требует Content-Type: application/json → браузерная форма `text/plain` (без preflight) с cookie жертвы выполняет JSON-тело.
→ Эксплуатация: вредоносная страница → скрытая форма → `POST /api/v1/assistant/design` за счёт жертвы (CPU/LLM-бюджет + запись в память, в связке с №1 — в чужой проект).
→ Патч: перевести на `authMutating` + в decodeJSON требовать JSON Content-Type для непустых тел.
→ Тест: text/plain-JSON с валидной cookie → 403.
→ OWASP A07 / CWE-352.

### 8. [internal/transport/http/assistant.go:98-100] | MEDIUM | Внутренняя err-цепочка в JSON-теле
→ `wrapped := fmt.Sprintf("assistant: %v", err)` → `writeError(..., wrapped)` — детали (URL провайдера, имена сервисов) клиенту. Разведка для №6/№7.
→ Патч: телу — фиксированный текст, цепочка — только в slog.
→ Тест: сбой модели → body без `assistant:`/URL.
→ CWE-209.

### 9. [internal/infrastructure/payments/stripe.go:161-166] | MEDIUM | Строгий парсинг подписи ломает ротацию
→ `len(parts) != 2` → при нескольких `v1=` в заголовке (ротация ключей Stripe) ВСЕ вебхуки отвергаются → окно простоя подтверждений оплат.
→ Патч: разобрать все пары, принять совпадение ЛЮБОЙ v1 с любым из сконфигурированных секретов.
→ Тест: `t=1,v1=old,t=2,v1=new` + новый секрет → verified.
→ CWE-754.

### 10. [internal/transport/http/router.go:235-236] | MEDIUM | WS без лимитов
→ `GET /ws`, `/ws/admin` — голый HandleFunc без limitRate; upgrade плодит pumps+hub-записи без бюджета на клиента.
→ Эксплуатация: флуд upgrade с валидной сессией → рост горутин/памяти хаба.
→ Патч: cap коннектов на IP+user + счётчик сообщений в readPump.
→ Тест: N+1 коннект → reject; флуд сообщений → drop.
→ CWE-400.

### 11. [internal/transport/http/internal_only.go:12-20,30-51] | MEDIUM, ТРЕБУЕТ ПРОВЕРКИ топологии | InternalOnly открыт за LB
→ Разрешён ВЕСЬ RFC1918, IP только из RemoteAddr. За ALB/nodePort RemoteAddr = IP балансировщика/ноды (внутри RFC1918) → `/metrics`, `/debug/pprof` фактически публичны для всех, кто достучался до пода.
→ Проверка: ingress/SG — доходит ли внешний клиент до пода напрямую? Если да — закрыть SG/NetworkPolicy или pin XFF от ingress.
→ CWE-284 / CWE-668.

### 12. Отсутствие NetworkPolicy | MEDIUM | deploy/ — пусто по запросу
→ Проверено: в deploy/ нет ни одной NetworkPolicy. Pod lockdown (runAsNonRoot/seccomp/no-privesc — values.yaml:35-54 ✓) есть, сегментации трафика нет: скомпрометированный фронт ходит в БД/Redis напрямую на сетевом уровне.
→ Патч: default-deny + allowlist (api→postgres/redis, фронты→только api, dns).
→ CIS Benchmark 5.3.

### 13. Нет удаления conversation-memory | MEDIUM | GDPR/152-ФЗ
→ Prune по TTL есть (cmd/api/main.go:227-248 ✓), но пользовательского удаления (self-service «забыть меня») и каскада при удалении юзера не найдено. Запрос на удаление сегодня — ручная операция в БД.
→ Патч: `DELETE /api/v1/assistant/memory` (по user/project) + ON DELETE CASCADE + runbook.
→ GDPR Art.17 / 152-ФЗ.

### 14. Нет hard-лимита LLM-бюджета | MEDIUM | Счёт открыт
→ Per-user 200 req/min есть (middleware_auth.go:172-180 ✓ — смягчает), но месячного/глобального cap на расход OpenRouter нет; free-tier мультиаккаунты обходят per-user лимит. Один скрипт с 100 аккаунтами = неконтролируемый счёт.
→ Патч: глобальный дневной/месячный бюджет + circuit breaker (превышение → только local-бэкенд) + алерт; fingerprinting массовых регистраций — отдельно.
→ OWASP LLM10 / CWE-400.

### 15. [deploy/terraform/environments/production/main.tf:19-21] + [.github/workflows/ci.yml: нет permissions] | LOW | Инфра-гигиена
→ DynamoDB-lock для tfstate закомментирован → параллельные apply роняют стейт. ci.yml без top-level `permissions:` → GITHUB_TOKEN с дефолтными (широкими) правами.
→ Патч: создать таблицу `stair-platform-tf-lock` + раскомментировать; `permissions: contents: read` в ci.yml.
→ CIS / CWE-732.

## Проверено и НЕ подтвердилось (честно)
recovery.go:32 — generic 500 без err; FTS — параметризованный plainto_tsquery (ai_repo.go:316); WS query-token отклоняется (websocket_handler.go:64-70) + role-gate /ws/admin; Sprintf-SQL — нет; dangerouslySetInnerHTML — нет в обоих фронтах; VITE_* — контакты+DSN (публичное по дизайну); SHA-pinning — на месте; сессии — httpOnly cookie (кража через XSS затруднена); tenant_id — во всех просмотренных SQL.

## Вердикт: УСЛОВНО НЕТ — клиентам пока нельзя
Блокеры показа: №1 (IDOR памяти), №2 (токены в Sentry), №3 (потеря платежа). Остальное — в 7-дневный план. RAG/Memory в текущем виде — только для доверенных внутренних пользователей.

## План 7 дней
1. Дни 1–2: №1 (member-check) + №2 (scrub regex/headers) + №7 (authMutating) — тесты-ловушки сначала.
2. Дни 3–4: №3 (транзакционный dedup/reconcile) + №4 (guard переходов) + №9 (парсинг подписей).
3. День 5: №5 (EXPIRE-DEL + алерт), №8 (generic errors), №10 (WS-лимиты).
4. День 6: №6 (safety-eval набор) + №14 (бюджетный breaker) + №13 (DELETE memory).
5. День 7: №11 (топология/SG), №12 (NetworkPolicy), №15 (lock+permissions) + повторный прогон всего.

## Тесты в первую очередь
IDOR-B-не-член→403; Sentry-payload без session/csrf; webhook crash→retry применяется; completed→expired не регрессирует; EXPIRE-fail→DEL; text/plain-POST→403; safety-injection→отказ; WS-флуд→drop.

## Стоимость AI (оценка)
Один Ask: эмбеддинг запроса (~$0.00002) + top5 чанков в контекст (~2–4k токенов) + ответ mini-класса ≈ **$0.001–0.003**; сильной моделью ≈ **$0.01–0.03**. 1000 активных юзеров × 10 Ask/мес: **$10–30** (mini) / **$100–300** (сильная). Узкие места: latency OpenRouter (хоп), отсутствие кеша одинаковых Ask (каждый платится), engine/variation CPU (локально 742с в race — на проде следить), WS-hub память при флуде (№10).
