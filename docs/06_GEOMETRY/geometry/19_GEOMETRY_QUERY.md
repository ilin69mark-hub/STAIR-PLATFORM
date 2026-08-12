# STAIR PLATFORM

Document: 19_GEOMETRY_QUERY.md

ID: ENG-GEO-0020

Status: APPROVED

---

# Purpose

Geometry Query предоставляет единый интерфейс поиска и получения геометрических объектов.

Geometry Query использует Graph Query и предоставляет специализированный API для работы с геометрией.

---

# Supported Queries

Find Vertex

Find Edge

Find Face

Find Solid

Find Component

Find Assembly

Find Sketch

Find Feature

Find by Material

Find by Layer

Find by Metadata

Find by Bounding Box

---

# Spatial Queries

Nearest Object

Inside Volume

Intersects

Contains

Touches

Distance

Ray Cast

Bounding Box Search

---

# Semantic Queries

Structural Elements

Load Bearing Components

Fasteners

Manufacturing Parts

Reference Geometry

Construction Geometry

---

# Query Output

Node Reference

Geometry Object

Topology Object

Semantic Object

Measurements

Metadata

---

# Requirements

Все запросы должны использовать Graph Platform.

Прямой доступ к внутреннему хранилищу Geometry запрещен.

Поддерживается потокобезопасное выполнение.

---

# Performance Goals

- индексированный поиск;
- пространственные индексы;
- ленивое получение данных;
- повторное использование кэша.

---

# Acceptance Criteria

- единый Geometry Query API;
- поддержка пространственного поиска;
- интеграция с Graph Query;
- масштабируемость на большие модели.

---

APPROVED