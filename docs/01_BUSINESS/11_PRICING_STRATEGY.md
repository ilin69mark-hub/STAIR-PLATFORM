# STAIR PLATFORM

**Document:** 11_PRICING_STRATEGY.md

**Document ID:** BUS-0012

**Version:** 1.0.0

**Status:** APPROVED

**Author:** Project Team

**Last Updated:** 2026-08-04

---

# 1. Purpose

Настоящий документ определяет стратегию ценообразования Stair Platform.

Документ используется для:

- формирования тарифов;
- разработки Billing System;
- лицензирования;
- монетизации функций;
- расчета Unit Economics.

Pricing Strategy должна обеспечивать гибкость, масштабируемость и возможность изменения тарифов без изменения программного кода.

---

# 2. Pricing Principles

При разработке модели ценообразования используются следующие принципы:

- Value-Based Pricing;
- Subscription First;
- Feature-Based Licensing;
- Predictable Pricing;
- Transparent Billing;
- Scalable Plans.

---

# 3. Pricing Model

Основной способ монетизации —

подписка (Subscription).

Дополнительно поддерживаются:

- Pay-as-you-Go;
- AI Credits;
- Enterprise License;
- Professional Services.

---

# 4. Licensing Model

Доступ к системе определяется лицензией.

Лицензия предоставляет доступ не к ролям, а к возможностям платформы.

Пример:

```
License

↓

Capabilities

↓

Features

↓

Limits
```

---

# 5. Billing Periods

Поддерживаются:

- Monthly;
- Quarterly;
- Annual.

Дополнительно:

- Trial;
- Custom Enterprise Contract.

---

# 6. Pricing Dimensions

Стоимость определяется несколькими параметрами.

### Organization

- количество организаций.

---

### Users

- число пользователей.

---

### Projects

- число активных проектов.

---

### Storage

- объем хранения файлов.

---

### AI Usage

- количество AI-запросов;
- AI-кредиты.

---

### API

- лимиты вызовов API.

---

### Rendering

- количество рендеров.

---

### Manufacturing

- экспорт производственной документации.

---

# 7. Capability-Based Pricing

Каждая возможность платформы лицензируется отдельно.

Пример.

| Capability | Basic | Professional | Enterprise |
|------------|-------|--------------|------------|
| Stair Designer | ✔ | ✔ | ✔ |
| Geometry Engine | ✔ | ✔ | ✔ |
| Solver | ✔ | ✔ | ✔ |
| Rendering | — | ✔ | ✔ |
| AI Assistant | — | ✔ | ✔ |
| Manufacturing | — | ✔ | ✔ |
| API | — | Limited | Unlimited |
| Integrations | — | Basic | Advanced |
| Analytics | — | Basic | Advanced |

---

# 8. Usage Limits

Для каждого тарифа определяются ограничения.

Например:

- количество пользователей;
- количество проектов;
- объем хранения;
- AI Credits;
- API Requests;
- Rendering Jobs.

---

# 9. Enterprise Pricing

Enterprise-клиенты получают:

- индивидуальное ценообразование;
- выделенную инфраструктуру;
- SLA;
- расширенную поддержку;
- SSO;
- интеграции;
- White Label;
- персонального менеджера.

---

# 10. Discounts

Поддерживаются:

- Annual Discount;
- Partner Discount;
- Educational License;
- Promotional Campaigns.

Все скидки управляются через Billing и не требуют изменения кода.

---

# 11. Billing Events

Основные события системы:

- Trial Started;
- Trial Ended;
- Subscription Created;
- Subscription Renewed;
- Subscription Cancelled;
- Payment Received;
- Payment Failed;
- Capability Upgraded;
- Capability Downgraded;
- AI Credits Purchased.

---

# 12. Pricing KPIs

Контроль эффективности выполняется по следующим показателям:

- MRR;
- ARR;
- ARPU;
- Churn;
- Expansion Revenue;
- LTV;
- CAC;
- Trial Conversion Rate.

---

# 13. Dependencies

Incoming:

- BUSINESS_MODEL
- REVENUE_MODEL

Outgoing:

- SALES_STRATEGY
- FINANCIAL_MODEL
- BILLING MODULE
- AUTHORIZATION
- FEATURE FLAGS

---

# 14. Acceptance Criteria

Документ считается завершенным, если:

- определена стратегия ценообразования;
- определены модели лицензирования;
- определены способы монетизации;
- определены ограничения тарифов;
- определены события Billing;
- определены KPI.

---

# 15. Version History

| Version | Date | Description |
|----------|------|-------------|
|1.0.0|2026-08-04|Initial version|

---

# 16. Approval

APPROVED