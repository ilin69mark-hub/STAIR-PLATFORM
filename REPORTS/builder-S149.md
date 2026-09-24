# S-149 — Топология internal + NetworkPolicy + гигиена (S-141 №11, №12, №15, день 7)

## Вердикт

**S-149 ЗАКРЫТ (код).** Три находки: ci least-privilege + internal-CIDR через env +
NetworkPolicy-шаблон. Остаток — действия человека с AWS/кластером (runbook ниже).
Self-execute @swarm, режим build-and-plan. Коммит в `dev/swarm/s141-fixes`.

## №15 — Инфра-гигиена (LOW, CIS/CWE-732)

- `ci.yml`: top-level `permissions: contents: read` + job `build` поднимает
  `packages: write` (GHCR-пуш) и `actions: write` (gha build-кеш). Остальные джобы
  (lint/test/e2e/k6/security-scan) — только чтение: checkout/setup-go/сканы без
  записей (SARIF-аплоада нет). YAML-валидация `yaml.safe_load` + сверка job-прав.
- tf-lock: код НЕ тронут осознанно — раскоммент `dynamodb_table` без созданной
  таблицы роняет `terraform init` всем. Runbook человеку (AWS, region us-east-1):
  ```sh
  aws dynamodb create-table --table-name stair-platform-tf-lock \
    --attribute-definitions AttributeName=LockID,AttributeType=S \
    --key-schema AttributeName=LockID,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST --region us-east-1
  # затем раскомментировать dynamodb_table в
  # deploy/terraform/environments/production/main.tf:21
  ```

## №11 — InternalOnly открыт за LB (MEDIUM, требует проверки топологии)

- Код: `STAIR_INTERNAL_CIDRS` (comma-separated) переопределяет дефолтный
  RFC1918-список — за LB можно запинить только pod-CIDR кластера вместо «всего
  RFC1918». Невалидные записи — skip+warn; env без валидных CIDR — откат к
  дефолту с error (опечатка не открывает/закрывает молча). Парсинг на запрос
  осознанно (пути редкие, env без рестарта, тесты без кеша).
- Тесты: `TestInternalOnlyCIDRsFromEnv` (pin 10.42/16: свой → 200, чужой
  RFC1918 → 403), `TestInternalOnlyInvalidEnvFallsBack`.
- Runbook человеку (проверка топологии, staging):
  ```sh
  # С внешней сети (не из кластера): всё ниже должно быть 403/таймаут
  curl -s -o /dev/null -w "%{http_code}\n" https://api.staging/metrics
  curl -s -o /dev/null -w "%{http_code}\n" https://api.staging/swagger
  # Доходит напрямую до пода? Если да — закрыть SG ingress (только от LB) и/или
  # перейти на ClusterIP + STAIR_INTERNAL_CIDRS=<pod-CIDR>, затем включить NP (№12).
  ```

## №12 — Нет NetworkPolicy (MEDIUM, CIS 5.3)

- `deploy/helm/stair-platform/templates/networkpolicy.yaml`: две политики на
  поды релиза — `<fullname>-default-deny` (Ingress+Egress deny-all) и
  `<fullname>-allow` (вход :service.port из релиза + ingressCIDRs; выход DNS
  :53 kube-system/kube-dns, DB `databaseCIDRs:databasePort`, Redis
  `redisCIDRs:redisPort`, HTTPS 0.0.0.0/0:443 флагом `egressInternet` для
  Stripe API + OpenRouter, плюс `extraEgressCIDRs`).
- `values.yaml`: `networkPolicy.*` (по умолчанию `enabled: false` — корректные
  CIDR знает только оператор; default-deny с пустыми CIDR сломал бы прод на
  первом upgrade; включение staging→прод). При `enabled` без CIDR — fail-fast
  (`fail` с понятным текстом).
- Валидация: `helm lint` 0 failed; `helm template` (off → 0 NP; on+CIDR → 2 NP,
  deny без rules, allow: ingress from=2, egress=4, DB CIDR/порт верны);
  rendered-YAML парсится `yaml.safe_load_all`. Включение: staging → сверка
  коннектов (БД/Redis/PSP/LLM) → прод.

## Гейты

- `gofmt` чист; http-пакет ok (включая 2 новых internal-only теста).
- helm lint/template — выше. K8s-apply не гонялся (нет кластера) — только статика.
- Полный `go test -race -p 1 ./...` — в фоне, результат дописать координатору.

## Остаток человеку (день 7+)

1. DynamoDB lock-таблица + раскоммент (команда выше).
2. Топология staging (curl-проверки выше) + `STAIR_INTERNAL_CIDRS` в прод-values.
3. Включение NP staging→прод.
4. Prometheus-алерты: `rate_limit_fail_open_total` (S-147),
   `stair_assistant_llm_budget_denied_total` (S-148).
