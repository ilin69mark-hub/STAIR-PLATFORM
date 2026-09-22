
---

# `19_INFRASTRUCTURE/15_BACKUP_INFRASTRUCTURE.md`

```markdown
# STAIR PLATFORM

Document: 15_BACKUP_INFRASTRUCTURE.md

ID: INFRA-0015

Status: APPROVED

---

# Purpose

Определяет Backup Infrastructure.

---

# Backup Targets

PostgreSQL

Object Storage

Critical Configuration

Infrastructure State

Migration Metadata

---

# Backup Types

Full Backup

Incremental Backup

Snapshot

Point-in-Time Recovery where supported

---

# Backup Flow

```text
Production
    │
    ├── Database ──────┐
    ├── Storage ───────┤
    └── Infrastructure ┤
                       ▼
                    Backup
                       │
                       ▼
                Protected Storage
```

---

# RPO / RTO (S-128)

Формат — `pg_dump -Fc` (custom) + `.sha256` + `.meta`, скрипты
`scripts/db-backup.sh` / `scripts/db-restore.sh`. WAL-архивации и PITR
нет — точка восстановления = последний дамп.

- RPO: интервал cron (прод: ежедневно → ≤ 24h потерь).
- RTO: ~15–30 мин (restore в пустую БД + проверки connectivity/migrate).
- Проверено (S-128): `scripts/db-backup-restore-check.sh` — backup →
  restore в отдельную БД → сверка 23 таблиц и строк (users/projects) → OK.

# Retention (S-128)

`BACKUP_RETENTION_DAYS` (default 30, `0` — не удалять): после успешного
дампа `db-backup.sh` удаляет `<db>_*.dump` старше N дней вместе с
`.sha256`/`.meta`. Некорректное значение — warning + пропуск prune
(бэкап не страдает).

# Schedule (staging/prod — ставит человек)

```cron
# Ежедневно 03:00 UTC, хранить 30 дней.
0 3 * * *  cd /srv/stair-platform && BACKUP_DIR=/var/backups/stair BACKUP_RETENTION_DAYS=30 ./scripts/db-backup.sh >>/var/log/stair-backup.log 2>&1
```

Ежемесячно: прогон `db-backup-restore-check.sh` на staging (обратимость),
результат — в операционный журнал. Cron/retention на staging — остаток
S-128 за человеком (нужен доступ к хосту).