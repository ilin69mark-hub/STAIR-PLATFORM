# STAIR PLATFORM

**Document:** 16_BUSINESS_METRICS.md

**Document ID:** BUS-0017

**Version:** 1.0.0

**Status:** APPROVED

**Author:** Project Team

**Last Updated:** 2026-08-04

---

# 1. Purpose

Настоящий документ определяет систему бизнес-метрик Stair Platform.

Документ используется для:

- оценки эффективности бизнеса;
- анализа роста продукта;
- принятия управленческих решений;
- мониторинга финансовых показателей;
- анализа пользовательской активности;
- оценки качества сервиса.

Все ключевые показатели должны рассчитываться автоматически на основании данных платформы.

---

# 2. Business Metrics Framework

Метрики разделяются на следующие категории:

- Financial Metrics;
- Growth Metrics;
- Sales Metrics;
- Marketing Metrics;
- Product Metrics;
- Customer Success Metrics;
- Operational Metrics;
- AI Metrics;
- Platform Metrics.

---

# 3. Financial Metrics

## MET-001

### Monthly Recurring Revenue (MRR)

Описание

Сумма ежемесячной регулярной выручки.

Формула

```
Σ Monthly Subscriptions
```

Целевое значение

Рост каждый месяц.

Источник данных

Billing Module.

---

## MET-002

### Annual Recurring Revenue (ARR)

Формула

```
MRR × 12
```

---

## MET-003

### Average Revenue Per User (ARPU)

Формула

```
Revenue / Active Customers
```

---

## MET-004

### Customer Lifetime Value (LTV)

Формула

```
ARPU × Lifetime
```

---

## MET-005

### Customer Acquisition Cost (CAC)

Формула

```
Marketing + Sales / New Customers
```

---

# 4. Sales Metrics

Основные показатели продаж.

- Lead Conversion Rate;
- Trial Conversion Rate;
- Win Rate;
- Average Deal Size;
- Sales Cycle Duration;
- Renewal Rate.

---

# 5. Marketing Metrics

Контроль эффективности маркетинга.

- Website Visitors;
- Organic Traffic;
- Paid Traffic;
- Marketing Qualified Leads (MQL);
- Sales Qualified Leads (SQL);
- Cost Per Lead (CPL);
- Return on Marketing Investment (ROMI).

---

# 6. Product Metrics

Основные показатели продукта.

- Daily Active Users (DAU);
- Weekly Active Users (WAU);
- Monthly Active Users (MAU);
- Feature Adoption Rate;
- Activation Rate;
- Time To Value;
- Retention Rate.

---

# 7. Customer Success Metrics

Показатели удовлетворенности клиентов.

- Net Promoter Score (NPS);
- Customer Satisfaction (CSAT);
- Customer Effort Score (CES);
- Churn Rate;
- Expansion Revenue.

---

# 8. Platform Metrics

Контроль работы платформы.

- API Requests;
- Active Organizations;
- Active Projects;
- Storage Usage;
- Rendering Jobs;
- Solver Executions;
- Generated Documents.

---

# 9. AI Metrics

Показатели AI-модуля.

- AI Requests;
- AI Response Time;
- AI Success Rate;
- AI Acceptance Rate;
- AI Cost Per Request;
- Average Tokens;
- Model Distribution.

---

# 10. Operational Metrics

Контроль эксплуатации.

- Deployment Frequency;
- Lead Time;
- Mean Time To Recovery (MTTR);
- Mean Time Between Failures (MTBF);
- Incident Count;
- Availability;
- Error Rate.

---

# 11. KPI Dashboard

Основная панель руководителя должна содержать:

## Business

- MRR;
- ARR;
- LTV;
- CAC;
- Profit.

---

## Product

- MAU;
- Retention;
- Feature Adoption;
- Active Projects.

---

## Customers

- Churn;
- NPS;
- CSAT.

---

## Platform

- Uptime;
- API;
- AI Usage;
- Infrastructure Cost.

---

# 12. Alert Thresholds

Для каждой метрики определяются:

- Target;
- Warning;
- Critical.

При достижении пороговых значений система должна автоматически формировать уведомления.

---

# 13. Reporting

Поддерживаются следующие отчеты:

- Daily Report;
- Weekly Report;
- Monthly Report;
- Quarterly Report;
- Annual Report.

---

# 14. Data Sources

Источники данных.

- Billing;
- CRM;
- Analytics;
- AI Engine;
- Backend Services;
- Frontend Analytics;
- Monitoring;
- Database.

---

# 15. Dependencies

Incoming

- BUSINESS_MODEL
- REVENUE_MODEL
- PRODUCT

Outgoing

- UNIT_ECONOMICS
- FINANCIAL_MODEL
- ANALYTICS
- DASHBOARDS

---

# 16. Acceptance Criteria

Документ считается завершенным, если:

- определены категории метрик;
- определены KPI;
- определены источники данных;
- определены отчеты;
- определены пороги оповещения.

---

# 17. Version History

| Version | Date | Description |
|----------|------|-------------|
|1.0.0|2026-08-04|Initial version|

---

# 18. Approval

APPROVED