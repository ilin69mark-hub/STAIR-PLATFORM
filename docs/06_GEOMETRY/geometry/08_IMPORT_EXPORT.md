# STAIR PLATFORM

Document: 08_IMPORT_EXPORT.md

ID: ENG-GEO-0009

Status: APPROVED

---

# Supported Formats

Разделено на два независимых набора: визуальный экспорт и экспорт для
станка. Это разные реестры в коде, и раньше документ их смешивал.

## Визуальный экспорт — `cad.Write` (`internal/infrastructure/cad/cad.go`)

DXF

STL

SVG

Ровно эти три. Формат вне списка возвращает ошибку `cad: unsupported format`.

## Экспорт для станка — `cad.WriteCNC` (`internal/infrastructure/cad/cnc_export.go`)

STEP

IGES

OBJ

3MF

CSV

JSON

XML

## Прочее

PDF — коммерческое предложение (`frontend/src/lib/proposal.ts`). Чертежная
часть идёт вектором, 3D — растр (S-177).

## Не реализовано

IFC и GLTF/GLB в коде отсутствуют полностью: ни экспортера, ни упоминания
в реестрах. Раньше документ выдавал их за поддерживаемые.

---

# Import Pipeline

File

↓

Parser

↓

Semantic Converter

↓

Topology

↓

Geometry

↓

Project

---

# Export Pipeline

Project

↓

Geometry

↓

Topology

↓

Semantic Model

↓

Writer

↓

File

---

# Rules

Все форматы используют единый внутренний канонический формат модели.

---

APPROVED