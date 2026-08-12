# STAIR PLATFORM

Document: 02_STORAGE_MODEL.md

ID: STG-0002

Status: APPROVED

---

# Purpose

Определяет модель хранения файлов платформы.

---

# Storage Object

Storage Object

↓

Metadata

↓

Binary Content

↓

Checksum

↓

Version

↓

Access Policy

↓

History

---

# Object Types

Engineering File

Drawing

Image

Document

Export

Import

Attachment

AI Artifact

Backup

---

# Metadata

UUID

Filename

Mime Type

Size

Checksum

Revision

Owner

Created At

---

# Rules

Файл является Immutable.

Изменение файла создает новую версию.

Физическое удаление запрещено.

---

# Acceptance Criteria

Каждый файл имеет Metadata и Version.

---

APPROVED