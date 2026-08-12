# STAIR PLATFORM

Document: 15_PRICE_HISTORY.md

ID: PRC-0016

Status: APPROVED

---

# Purpose

Price History хранит историю всех изменений стоимости проекта.

---

# Objectives

- аудит коммерческих изменений;
- воспроизводимость расчета;
- анализ динамики стоимости;
- восстановление предыдущих расчетов.

---

# History Entry

History ID

Revision

Timestamp

Author

Operation

Previous Price

New Price

Currency

Reason

Correlation ID

---

# Recorded Operations

Cost Update

Margin Update

Discount Update

Tax Update

Currency Update

Validation

Commercial Offer

Rollback

---

# Navigation

Previous Revision

Next Revision

Compare

Restore

Audit

---

# Rules

История неизменяема.

Удаление записей запрещено.

Каждое изменение создает новую Revision.

---

# Acceptance Criteria

- полная история изменений;
- поддержка восстановления;
- трассируемость всех коммерческих операций.

---

APPROVED