# STAIR PLATFORM

**Document:** EDR-0026_Storage_Integration.md

**ID:** EDR-0026

**Status:** APPROVED

**Author:** Project Team

**Date:** 2026-08-15

**Category:** Integrations

---

# 1. Purpose

Документ фиксирует Storage-интеграцию (Phase E, E5) — абстракцию объектного
хранилища платформы с двумя бэкендами: локальная файловая система и
S3-совместимое хранилище (AWS S3 / MinIO) на чистой stdlib (подпись
SigV4 без внешних SDK, DEV-0009).

Storage даёт платформе персистентное хранение бинарных документов
(экспорты CAD из E1, в перспективе — отчёты, вложения):

1. **`ObjectStore`** — `internal/infrastructure/storage`: интерфейс
   `Put/Get/Delete`, фабрика по `STAIR_STORAGE_BACKEND` (`filesystem`|`s3`).
2. **Filesystem-бэкенд** — хранение в каталоге (`STAIR_STORAGE_DIR`).
3. **S3-бэкенд** — PUT/GET/DELETE через SigV4 (`STAIR_S3_*` переменные;
   поддержка path-style для MinIO).
4. **Storage-сервис** — `internal/application/storage`: ключи объектов
   скоупятся по tenant (`{tenant}/{category}/{...}`); хранение экспорта CAD.
5. **Транспорт** — сохранение экспорта в хранилище и получение по ключу.

---

# 2. Related Artifacts

- ROADMAP-0011 (Phase E — Integrations: Storage)
- EDR-0022 (CAD Export — экспортные документы, которые сохраняются в Storage)
- ADR-0006 (Layered Architecture)
- DEV-0009 (Offline builds — только stdlib, S3 через SigV4 без SDK)
- ADR-0008 (Единицы СИ; контент хранится байтами, формат в ключе)

---

# 3. Model

## 3.1 `ObjectStore` (`internal/infrastructure/storage/storage.go`)

```go
// ObjectStore — абстракция объектного хранилища (EDR-0026 §3.1).
// Ключ — иерархический путь ({tenant}/{category}/{filename}). Реализации
// инвариантны: ключи не должны содержать ".." и ведущих слэшей.
type ObjectStore interface {
    Put(ctx context.Context, key string, data []byte, contentType string) error
    Get(ctx context.Context, key string) ([]byte, error)
    Delete(ctx context.Context, key string) error
}

// ErrNotFound — объект не найден.
var ErrNotFound = errors.New("storage: not found")

// ErrInvalid — некорректный ключ (пустой, "..", вед. слэш).
var ErrInvalid = errors.New("storage: invalid key")

// NewObjectStore выбирает бэкенд по backend ("filesystem"|"s3") и параметрам.
func NewObjectStore(backend string, o Options) (ObjectStore, error)
```

| Опция | Переменная | По умолчанию |
|-------|-----------|--------------|
| backend | `STAIR_STORAGE_BACKEND` | `filesystem` |
| filesystem root | `STAIR_STORAGE_DIR` | `./storage` |
| s3 endpoint | `STAIR_S3_ENDPOINT` | обязателен для s3 |
| s3 bucket | `STAIR_S3_BUCKET` | обязателен |
| s3 region | `STAIR_S3_REGION` | `us-east-1` |
| s3 access key | `STAIR_S3_ACCESS_KEY` | обязателен |
| s3 secret key | `STAIR_S3_SECRET_KEY` | обязателен |
| s3 path-style | `STAIR_S3_PATH_STYLE` | `true` (MinIO-совместимо) |

## 3.2 Filesystem-бэкенд (`internal/infrastructure/storage/fs.go`)

- Ключ нормализуется (`path.Clean`), отклоняется при `..`/вед. слэше.
- Путь `root/<key>`; `Put` создаёт каталоги, `Get` читает, `Delete` удаляет.
- Ключ экранируется от выхода за `root` (SymlinkEval/чистка + проверка prefix).

## 3.3 S3-бэкенд (`internal/infrastructure/storage/s3.go`)

- SigV4 (`internal/infrastructure/storage/sigv4.go`) на stdlib: canonical
  request → string-to-sign → HMAC-цепочка ключей → `Authorization` заголовок.
- `x-amz-content-sha256` = hex(sha256(body)) (для PUT; GET — пустой hash).
- PUT/GET/DELETE к `{endpoint}/{bucket}/{key}`; `Host` заголовок — endpoint
  host (path-style: bucket в пути).
- Не-2xx статус — ошибка с `Code` из тела ошибки S3.

## 3.4 Storage-сервис (`internal/application/storage`)

```go
type ObjectStore interface { /* Put/Get/Delete как в §3.1 */ }

type Service struct { store ObjectStore }

// SaveExport сохраняет экспорт проекта и возвращает ключ объекта
// {tenant}/{project}/exports/{category}/{filename}.
func (s *Service) SaveExport(ctx, tenantID, projectID, category, filename string, data []byte, contentType string) (string, error)

// Load возвращает данные и content-type по ключу (переданный клиентом).
func (s *Service) Load(ctx, key string) ([]byte, string, error)

// Delete удаляет объект по ключу.
func (s *Service) Delete(ctx, key string) error
```

- Ключ конструируется сервисом из tenant (скоуп): внешний клиент не может
  положить объект в чужой tenant.
- `Load` по ключу: транспорту нужно проверить префикс tenant пользователя
  (не давать читать чужие объекты) — либо `Service.Load` принимает tenantID
  и проверяет префикс ключа.

## 3.5 Транспорт

| Method | Path | Auth | Описание |
|--------|------|------|----------|
| POST | `/api/v1/projects/{id}/export/cad/store?format=dxf\|stl\|svg` | owner/editor | Экспорт CAD (E1) + сохранение в Storage, возвращает `{key, content_type, size}` |
| GET | `/api/v1/storage/{key}` | auth (член tenant) | Получение объекта по ключу (скоуп tenant проверяется) |
| DELETE | `/api/v1/storage/{key}` | admin | Удаление объекта |

- `POST .../export/cad/store` переиспользует `ProjectService.ExportCAD` +
  `cad.Write` (E1), затем `storage.Service.SaveExport`; 201 — `{key,...}`;
  400 — неверный format; 403/404/422 — как в E1.
- `GET /api/v1/storage/{key}` — `Content-Type` из хранилища; 404 — нет
  объекта; 403 — ключ вне tenant пользователя.

---

# 4. Invariants

```
1. Ключи объектного хранилища иерархические и tenant-скоупed; ".." и
   ведущие слэши отклоняются (ErrInvalid).
2. Filesystem-бэкенд не позволяет запись за пределы root.
3. S3-подпись — SigV4 (HMAC-SHA256), реализация на чистой stdlib.
4. Секреты S3 не логируются и не попадают в JSON-ответы.
5. Хранилище не хранит никаких секретов платформы.
6. GET /storage/{key} скоупен по tenant (403 при чужом ключе).
```

---

# 5. Schema

Изменений БД нет: объекты адресуются ключами; метаданные экспортов при
необходимости — в будущем через существующие таблицы/аудит.

---

# 6. API

`POST /api/v1/projects/{id}/export/cad/store?format=dxf` → 201:

```json
{ "key": "<tenant>/<project>/exports/cad/project-<id>.dxf",
  "content_type": "application/dxf", "size": 1234 }
```

`GET /api/v1/storage/{tenant}/<project>/exports/cad/project-<id>.dxf` → 200
файл с `Content-Type`; 403 при чужом tenant; 404 при отсутствии.

---

# 7. Tests

1. `infrastructure/storage/sigv4_test.go` — эталонный вектор AWS SigV4
   (GET ListUsers) и собственная подпись PUT совпадает.
2. `infrastructure/storage/fs_test.go` — Put/Get/Delete; выход за root
   отклоняется; ErrNotFound.
3. `infrastructure/storage/s3_test.go` — httptest S3-сервер: PUT/GET/DELETE,
   подпись проходит, не-2xx → ошибка.
4. `application/storage` — SaveExport строит ключ с tenant-префиксом; Load
   проверяет скоуп; ErrInvalid для "..".
5. Транспорт — export/cad/store (201, 400, 403, 404) и storage GET
   (200/403/404); DELETE (204/404).

---

APPROVED