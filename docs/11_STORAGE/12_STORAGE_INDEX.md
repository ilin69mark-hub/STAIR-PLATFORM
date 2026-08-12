# STAIR PLATFORM

Document: 12_STORAGE_INDEX.md

ID: STG-0012

Status: APPROVED

---

# Purpose

Определяет индекс хранения файлов.

---

# Indexed Fields

UUID

Filename

Checksum

Revision

Project

Owner

Created At

Storage Class

Mime Type

Object Type

---

# Search

UUID

Filename

Revision

Checksum

Project

Tag

Metadata

---

# Rules

Индекс хранит только Metadata.

Binary не индексируется.

Индекс обновляется автоматически.

---

# Validation

Все объекты имеют индекс.

Индекс синхронизирован со Storage.

---

# Acceptance Criteria

Все объекты доступны через индекс.

---

APPROVED