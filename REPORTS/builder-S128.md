# S-128: Backup/restore — roundtrip OK + retention + RPO/RTO

**Вердикт:** DONE (локальная часть) — скрипты проверены roundtrip, retention реализован и протестирован, RPO/RTO задокументированы, запушено в `dev/swarm/ws-s115-s116`. Остаток staging (cron/retention на хосте) — за человеком, зафиксировано в доке.

## Что сделано
- Верификация (без правок — скрипты уже рабочие): `scripts/db-backup-restore-check.sh` на `stair-test-pg`/`stair_test` → `OK (backup and restore are reversible)`: 23/23 таблицы, users 29/29, projects 15/15 строк.
- `scripts/db-backup.sh` — новый `BACKUP_RETENTION_DAYS` (default 30, `0` — не удалять): prune `<db>_*.dump` старше N дней + `.sha256`/`.meta`; мусорное значение → warning + пропуск (бэкап цел).
- `docs/17_INFRASTRUCTURE/15_BACKUP_INFRASTRUCTURE.md` — RPO (≤24h, PITR нет — честно), RTO (~15–30 мин), retention, cron-пример, ежемесячный check на staging.

## Гейты (ops-зона, Go-код не тронут)
- `bash -n` — ok; живые прогоны: prune старого дампа (1 удалён, новый цел), `junk` → warning, `0` → пропуск.
- Roundtrip check — OK.

## Остаток человеку
Cron на staging-хосте + ежемесячный restore-check (нужен SSH-доступ). Записано в доке и на борде.

## Счётчик CI
Задача 15/20 после PR #51 (следующий CI-прогон после 20-й).
