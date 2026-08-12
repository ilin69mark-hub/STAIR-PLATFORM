# STAIR PLATFORM

Document: 03_MODEL_ROUTER.md

ID: AI-0003

Status: APPROVED

---

# Purpose

Определяет механизм выбора AI-моделей.

---

# Objectives

Выбор оптимальной модели.

Минимизация стоимости.

Минимизация задержек.

Повышение надежности.

---

# Supported Providers

OpenAI

Anthropic

Google

OpenRouter

Ollama

Local Models

ONNX Runtime

---

# Routing Criteria

Task Type

Latency

Model Availability

Context Size

Cost

Capabilities

Priority

---

# Fallback Strategy

Primary Model

↓

Secondary Model

↓

Local Model

↓

Failure

---

# Acceptance Criteria

Router автоматически выбирает наиболее подходящую модель.

---

APPROVED