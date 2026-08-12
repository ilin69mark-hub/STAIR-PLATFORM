# STAIR PLATFORM

Document: 03_OBJECT_STORAGE.md

ID: STG-0003

Status: APPROVED

---

# Purpose

Определяет объектное хранилище платформы.

---

# Supported Storage

Amazon S3

MinIO

Azure Blob

Google Cloud Storage

Local S3 Compatible

---

# Object Structure

Bucket

↓

Folder

↓

Object

↓

Version

↓

Metadata

---

# Bucket Categories

Projects

Geometry

Drawings

Manufacturing

Pricing

Documents

AI

Exports

Backups

Logs

---

# Object Naming

UUID

Revision

Checksum

Extension

---

# Access

Signed URL

Service Account

Internal API

---

# Acceptance Criteria

Все бинарные данные размещаются в Object Storage.

---

APPROVED