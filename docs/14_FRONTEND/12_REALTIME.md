# STAIR PLATFORM

Document: 12_REALTIME.md

ID: FE-0012

Status: APPROVED

---

# Purpose

Определяет архитектуру взаимодействия Frontend с платформой в реальном времени.

---

# Use Cases

Model Updates

Graph Updates

AI Streaming

Job Progress

Notifications

Collaboration

System Events

---

# Transport

WebSocket

Server-Sent Events

WebRTC (только при необходимости)

---

# Event Pipeline

Backend Event

↓

Event Gateway

↓

Realtime Transport

↓

Frontend Event Bus

↓

Context Update

↓

UI Update

---

# Event Types

Project Event

Geometry Event

Graph Event

AI Event

Manufacturing Event

System Event

---

# Rules

Realtime Events не изменяют состояние напрямую.

Событие передается соответствующему владельцу состояния.

---

# Reliability

Reconnect

Heartbeat

Timeout

Event Ordering

Duplicate Detection

---

# Acceptance Criteria

Frontend корректно восстанавливает realtime-соединение после временного отключения.

---

APPROVED