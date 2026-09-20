
---

# `19_INFRASTRUCTURE/10_INFRASTRUCTURE_AS_CODE.md`

```markdown
# STAIR PLATFORM

Document: 10_INFRASTRUCTURE_AS_CODE.md

ID: INFRA-0010

Status: APPROVED

---

# Purpose

Определяет Infrastructure as Code Strategy.

---

# Principle

Infrastructure должна быть описана декларативно и version-controlled.

---

# Managed Resources

Networking

Compute

Containers

Database Infrastructure

Cache

Queue

Storage

Load Balancer

DNS

Monitoring

Secrets Integration

---

# Repository

Infrastructure configuration хранится в version-controlled repository вместе с историей изменений.

---

# Environments

Каждый environment имеет отдельную configuration layer.

---

# Change Flow

```text
Change
 ↓
Review
 ↓
Validation
 ↓
Plan
 ↓
Approval
 ↓
Apply
 ↓
Verify

---

# S2-2: Единственный владелец инфраструктуры

Чтобы исключить конфликты двойного управления (terraform И helm меняют один
ресурс), каждый ресурс имеет ровно одного владельца:

| Владелец       | Ресурсы                                                              |
|----------------|----------------------------------------------------------------------|
| `terraform`    | Кластер (EKS), сеть (VPC/subnets), БД (RDS), namespace               |
| `helm` (CD)    | Deployment (api/worker), Service, ConfigMap, Secret, HPA, PDB, Ingress |

Правила (S2-2):
1. `deploy/terraform/**` НЕ объявляет `helm_release` и НЕ подключает helm-
   провайдер — приложение в terraform не разворачивается.
2. `deploy/helm/**` НЕ содержит terraform-провайдеров.
3. Имя версии образа (`image.tag`) и остальные values живут ТОЛЬКО в CD
   (`ci/cd.yml`), не в terraform — чтобы не было двух «лидеров» у релиза.
4. CI-гейт `scripts/check-infra-ownership.sh` (в `architecture-checks`)
   запрещает «пересечения»: падает, если в terraform появится helm-блок.