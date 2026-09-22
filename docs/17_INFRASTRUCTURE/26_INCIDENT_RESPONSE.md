
---

# `19_INFRASTRUCTURE/26_INCIDENT_RESPONSE.md`

```markdown
# STAIR PLATFORM

Document: 26_INCIDENT_RESPONSE.md

ID: INFRA-0026

Status: APPROVED

---

# Purpose

Инструкция дежурного: классификация, роли, митигация, откат, postmortem.
Связанные доки: 21_ALERTING (маршрутизация), 22_SLO_SLA (пороги),
25_INFRASTRUCTURE_DISASTER_RECOVERY (классы отказов),
15_BACKUP_INFRASTRUCTURE (RPO/RTO/retention, S-128).

---

# Severity

SEV-1 (критичный): API недоступен, утечка данных, 5xx > 5%.
Реакция ≤ 15 мин, митигация ≤ 1ч, postmortem ≤ 48ч.

SEV-2 (высокий): деградация (p95/p99 SLO, пул БД, CB open), 5xx 1–5%.
Реакция ≤ 1ч, митигация ≤ 4ч, postmortem ≤ 5 дней.

SEV-3 (умеренный): одиночные варнинги (память, горутины, rate-limit spike).
Реакция — следующий рабочий день, postmortem не обязателен.

---

# Roles

Incident Commander (IC): ведёт инцидент, принимает решения, единственный
голос наружу. Comms: статусы пользователям/стейкхолдерам. On-call:
первый респондент, исполняет runbook. Правило: IC ≠ исполнитель митигации
на SEV-1 (разделение внимания).

On-call ротация и контакты — заполняет человек (см. Critical contacts
в 18_PRODUCTION_LAUNCH_CHECKLIST): primary/secondary, телефон, escalation
через 15 мин без ответа.

---

# Flow

```text
Alert (Alertmanager → webhook, S5-1)
  ↓
Acknowledge (кто взял — пишет в операционный канал)
  ↓
Triage (SEV-класс по таблице выше)
  ↓
Mitigate (откат/масштаб/фича-флаг — сначала остановить кровотечение)
  ↓
Resolve (проверка SLO 30 мин → закрытие)
  ↓
Postmortem (blameless, ≤48ч для SEV-1: timeline, root cause, action items)
```

---

# Rollback

API/worker (Helm, immutable-теги sha — S2-3, откат безопасен):

```bash
helm history stair -n stair-platform
helm rollback stair -n stair-platform <REVISION>
```

Миграции БД — только вниз по версиям (up/down-пары в migrations/):

```bash
STAIR_DATABASE_URL="postgres://..." go run ./cmd/migrate -dir migrations -database "$STAIR_DATABASE_URL" -down
```

ВНИМАНИЕ: down-миграции могут терять данные — сначала дамп через
`scripts/db-backup.sh`. Потерянное восстанавливается из дампа
(`scripts/db-restore.sh`, RPO/RTO — 15_BACKUP_INFRASTRUCTURE).

Фронтенды (store/admin): redeploy предыдущего ghcr-образа
(тег = github.sha, не latest) через CD workflow_dispatch либо
`helm rollback` если фронты в чарте.

Проверка после отката: /health, p95 login (алерт StairLoginSlow, S-126),
5xx-доля, smoke login → quote → orders.

---

# On-call Handoff

Ротация — еженедельная, handoff checklist: открытые алерты, тихие
(inhibited) правила, известные деградации, ссылка на этот док.
Тишина пейджера > 30 дней — повод проверить, что алерты вообще
стреляют (тест StairApiDown на staging, см. 21_ALERTING).
```

---

# Acceptance Criteria

SEV-классификация, роли, rollback-команды и on-call handoff описаны;
закрывает 3 бокса Operations в 18_PRODUCTION_LAUNCH_CHECKLIST (S-129).
Имена дежурных и контакты — заполняет человек при запуске.
