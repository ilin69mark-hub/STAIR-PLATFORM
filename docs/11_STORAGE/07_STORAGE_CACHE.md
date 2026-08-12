# STAIR PLATFORM

Document: 07_STORAGE_CACHE.md

ID: STG-0007

Status: APPROVED

---

# Purpose

Определяет стратегию кэширования файлов.

---

# Cache Objects

Preview

Thumbnail

Mesh Cache

STEP Cache

PDF Preview

Image Preview

AI Preview

Temporary Export

---

# Cache Layers

Memory Cache

Redis

Disk Cache

CDN Cache

---

# Cache Rules

Кэш не является источником истины.

Кэш может быть полностью очищен.

Кэш автоматически перестраивается.

---

# Invalidation

Revision Changed

File Updated

Storage Deleted

Manual Cleanup

TTL Expired

---

# Acceptance Criteria

Удаление кэша не приводит к потере данных.

---

APPROVED