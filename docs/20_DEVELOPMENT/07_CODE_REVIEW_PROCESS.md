# STAIR PLATFORM

Document: 07_CODE_REVIEW_PROCESS.md

ID: DEV-0007

Status: APPROVED

---

# Purpose

Определяет процесс Code Review.

---

# Objective

Code Review направлен на повышение качества системы, а не только на поиск ошибок.

---

# Review Criteria

Проверяются:

* Correctness
* Architecture
* Readability
* Maintainability
* Security
* Testing
* Performance where applicable

---

# Review Checklist

* [ ] Код соответствует архитектуре
* [ ] Нет скрытых side effects
* [ ] Ошибки обрабатываются корректно
* [ ] Добавлены необходимые тесты
* [ ] Нет дублирования
* [ ] Документация обновлена при необходимости

---

# Reviewer Responsibilities

Reviewer оценивает изменение независимо от автора.

---

# Author Responsibilities

Автор отвечает за:

* описание изменения
* ответы на замечания
* актуальность тестов
* устранение найденных проблем

---

# Blocking Comments

Следующие замечания считаются blocking:

* нарушение архитектуры
* security issue
* data integrity issue
* отсутствие обязательных тестов
* breaking behavior без документации

---

# Approval

Merge допускается только после необходимого количества approvals согласно repository policy.

---

# Acceptance Criteria

Каждое значимое изменение проходит формальный Code Review.

---

APPROVED
