# STAIR PLATFORM

Document: 12_CNC_EXPORT.md

ID: MFG-0013

Status: APPROVED

---

# Purpose

CNC Export Engine преобразует производственную модель в форматы,
поддерживаемые станками и CAM-системами.

Engine не рассчитывает траектории инструмента.
Он предоставляет стандартизированные данные для CAM.

---

# Objectives

- экспорт производственных данных;
- поддержка различных CAD/CAM систем;
- сохранение геометрической точности;
- подготовка файлов для автоматизированного производства.

---

# Supported Formats

DXF

DWG

STEP

IGES

SVG

STL

OBJ

3MF

CSV

JSON

XML

Custom CNC Formats

---

# Export Content

Part Geometry

Contours

Holes

Reference Points

Coordinate System

Material

Thickness

Metadata

Revision

---

# Export Rules

Каждый экспортируемый файл содержит:

- Revision;
- единицы измерения;
- систему координат;
- сведения о материале;
- уникальный идентификатор детали.

---

# Validation

Перед экспортом проверяются:

- целостность геометрии;
- корректность размеров;
- отсутствие открытых контуров;
- совместимость формата.

---

# Output

CNC Package

Machine Files

Export Report

Validation Report

---

# Acceptance Criteria

- воспроизводимый экспорт;
- поддержка пакетного экспорта;
- отсутствие потери геометрических данных.

---

APPROVED