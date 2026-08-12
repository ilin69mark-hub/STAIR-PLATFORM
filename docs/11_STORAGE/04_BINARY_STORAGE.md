# STAIR PLATFORM

Document: 04_BINARY_STORAGE.md

ID: STG-0004

Status: APPROVED

---

# Purpose

Определяет правила хранения бинарных данных.

---

# Binary Types

STEP

DXF

DWG

OBJ

STL

GLTF

PNG

JPEG

PDF

ZIP

AI Models

AI Embeddings

Cache Files

---

# Binary Rules

Бинарные данные не изменяются.

Каждый Binary Object имеет Checksum.

Размер файла фиксируется.

Версия неизменяема.

---

# Integrity

SHA-256

Checksum Validation

Duplicate Detection

Corruption Detection

---

# Storage Policy

Binary хранится отдельно от Metadata.

Database хранит только ссылки.

---

# Acceptance Criteria

Все бинарные объекты проходят проверку целостности.

---

APPROVED