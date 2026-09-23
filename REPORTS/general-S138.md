# S-138: Пересоздание EKS-кластера терраформом (код + runbook, без apply)

**Вердикт:** код готов к ручному apply человеком; кластер пересоздаётся сразу на Kubernetes 1.35 через модуль `terraform-aws-modules/eks/aws ~> 20.0`, нод-группы и остальные модули не тронуты. `terraform plan/apply` НЕ запускался (нет AWS-доступа, terraform не установлен) — apply строго вручную по runbook ниже, после S-118 (GH secrets для CD-apply) либо локально с AWS-ключами.

## 1. Выбранные версии (+ почему)

| Компонент | Было | Стало | Почему |
|---|---|---|---|
| Kubernetes (`cluster_version`) | 1.28 (EOL) | **1.35** | На сентябрь 2026 в standard support только 1.34 / 1.35 / 1.36 (источник: docs.aws.amazon.com/eks/latest/userguide/kubernetes-versions.html). 1.34 выходит из standard support 02.12.2026 (через ~2 мес.), 1.36 выпущен 02.06.2026 — слишком свежий для прода (незрелые AMI/аддоны). 1.35 (EKS-релиз 27.01.2026, standard support до 27.03.2027) — зрелая середина с запасом поддержки ~18 мес. |
| Модуль `terraform-aws-modules/eks/aws` | 19.0.0 | **`~> 20.0`** (плавающий, резолвится в последний v20.x) | v19 держит только старые версии K8s по срокам поддержки AMI/аддонов. v20 — последний мажор с интерфейсом v19 (`cluster_name`, `cluster_version`, `eks_managed_node_groups` без переименований — проверено по примеру registry для 20.x и `UPGRADE-20.0.md`), т.е. drop-in замена. v21 (latest 21.25.0, авг 2026) НЕ взят сознательно: требует AWS-провайдер v6 (`>= 6.52`) и переименовывает переменные (`name`, `kubernetes_version`, `compute_config`), что потянуло бы за собой весь стек. Точный патч не пиним — `~> 20.0` (`>= 20.0, < 21.0`) заберёт свежий v20.x на `init`. |
| AWS-провайдер | `~> 5.0` | **`>= 5.34, < 6.0`** | `>= 5.34` — документированный минимум модуля v20 (`UPGRADE-20.0.md`); потолок `< 6.0` удерживает совместимость с VPC-модулем 5.0.0 и своими модулями (все на AWS 5.x). Поздние v20.x просят ещё новее 5.x — резолвится автоматически внутри мажора. |
| `required_version` terraform | `>= 1.0` | **`>= 1.3`** | Минимум модуля v20 (нужен для `moved`-блоков и др.). |
| VPC-модуль | 5.0.0 | без изменений | Совместим с AWS 5.x, трогать не нужно. |
| Свои модули `postgres` / `kubernetes` | — | без изменений | Пины AWS `~> 5.0` / kubernetes `~> 2.0` совместимы, интерфейс не менялся. |
| Нод-группа `general` | t3.medium, 1–10 / desired 2 | без изменений | Pre-launch, нагрузка нулевая: дефолтных 2× t3.medium достаточно, автомасштабирование до 10 покрывает пик деплоя. Менять класс/число без метрик не на что — пересмотр после запуска по фактической утилизации. |

Проверки совместимости 1.35 со стеком (по release notes EKS): удаление cgroup v1 — не afecta, т.к. дефолтные AMI модуля (AL2023) используют cgroup v2; containerd 1.x EOL — модуль тянет свежие AMI с containerd 2.x; удалённый флаг kubelet `--pod-infra-container-image` касается только кастомных AMI — у нас дефолтные. Действий не требуется.

Добавлено `enable_cluster_creator_admin_permissions = true`: в v20 bootstrap прав создателя через aws-auth ConfigMap убран (дефолт `authentication_mode = API_AND_CONFIG_MAP`), без этого флага IAM-принципал, выполнивший apply, не получит доступ к новому кластеру (kubectl после `update-kubeconfig` будет запрещён).

## 2. Что изменено

- `deploy/terraform/environments/production/main.tf`:
  - `module.eks`: `version 19.0.0 → ~> 20.0`, `cluster_version "1.28" → "1.35"`, добавлен `enable_cluster_creator_admin_permissions = true`, комментарий с обоснованием S-138;
  - `terraform.required_version`: `>= 1.0 → >= 1.3`;
  - пин AWS-провайдера: `~> 5.0 → >= 5.34, < 6.0`.
- Больше ничего не тронуто: VPC, postgres (Aurora, `deletion_protection` по умолчанию true — БД переживёт пересоздание кластера), модуль `kubernetes` (только namespace, правило S2-2 соблюдено — чарт ставит CD, не terraform).

## 3. Проверки

- `terraform fmt -check` / `terraform validate` / `plan` — **НЕ выполнялись**: бинарника terraform в окружении нет (`which terraform` — not found), AWS-доступа нет. Вместо этого: статическая проверка (баланс `{}`/`[]`/`"` — ОК, все 4 module-блока на месте, значения пинов подтверждены парсингом файла).
- Человек перед apply обязан прогнать: `terraform fmt -check` (или `terraform fmt`), `terraform init -upgrade`, `terraform validate`, `terraform plan`.

## 4. Runbook apply (выполняет человек вручную)

> ⚠️ **Старый кластер `stair-platform-production` (1.28) будет УДАЛЁН и создан заново.** EKS API запрещает пропуск минорных версий при in-place update (1.28→1.35 напрямую отклонит), поэтому только recreate. Прод pre-launch, терять нечего; ворклоады и релизы после пересоздания поднимает CD-чарт заново. RDS (Aurora postgres) в recreate НЕ входит и не удаляется — но убедиться по `plan`, что в нём нет replace/delete.

Предусловия: AWS-ключи с правами на EKS/EC2/IAM/RDS, terraform `>= 1.3`, S3-бакет `stair-platform-terraform` существует; таблица DynamoDB `stair-platform-tf-lock` создана (partition key `LockID`, String) и строка `dynamodb_table` в backend-блоке раскомментирована (см. комментарий в main.tf).

```bash
cd deploy/terraform/environments/production

# 0. Сверить, что это та самая ветка/код S-138
git status --short   # только main.tf (+ этот отчёт)

# 1. Инициализация с S3-бэкендом и DynamoDB-локом
terraform init -upgrade

# 2. Формат + синтаксис
terraform fmt -check
terraform validate

# 3. План — ПРОВЕРИТЬ ГЛАЗАМИ:
#    - module.eks + нод-группы: replace/recreate (ожидаемо);
#    - module.postgres / aws_rds_cluster: NO changes (иначе СТОП);
#    - module.vpc: NO changes или in-place без замены (иначе СТОП).
terraform plan -out=tfplan-s138

# 4. Прицельно снести СТАРЫЙ кластер и namespace (БД и VPC не трогаем).
#    Важно: сначала kubernetes-ресурсы terraform'а, иначе namespace
#    застрянет без API-сервера.
terraform destroy -target=module.kubernetes -auto-approve
terraform destroy -target=module.eks -auto-approve

# 5. Создать новый кластер 1.35 на модуле v20
terraform apply tfplan-s138
# (план из шага 3 после destroy протухнет — перегенерить: terraform plan -out=tfplan-s138 && terraform apply tfplan-s138)

# 6. Подключиться и проверить
aws eks update-kubeconfig --region us-east-1 --name stair-platform-production
kubectl version --output=yaml   # server 1.35.x
kubectl get nodes               # 2× t3.medium Ready
kubectl get ns stair-platform   # namespace от модуля kubernetes

# 7. Приложение — через CD: S-118 (GH secrets) → запуск ci/cd.yml,
#    который ставит deploy/helm-чарт. Руками helm/chart НЕ ставить (S2-2).
```

Если шаг 4 страшно делать таргетами: альтернатива — `terraform taint` на ресурсы кластера + `apply` (пересоздание через замену), но при смене мажора модуля и версии K8s чистый `destroy -target + apply` предсказуемее.

## 5. Риски / откат

- **ВНИМАНИЕ: старый кластер удаляется.** Все ворклоады, LB/ingress-адреса, EBS-тома ворклоадов, временные данные в кластере — пропадут. Для pre-launch приемлемо; прод с трафиком так делать нельзя.
- План может показать замену VPC/БД (дрейф стейта vs новый провайдер/модуль) — тогда apply запрещён до разбора.
- В v20 удалён aws-auth сабмодуль из ядра: если в будущем понадобятся кастомные записи aws-auth — отдельный сабмодуль `terraform-aws-modules/eks/aws//modules/aws-auth ~> 20.0`. Сейчас не нужен (дефолтных access entries для нод-групп хватает, EKS создаёт их сам).
- Откат: новый кластер удаляется тем же кодом (`destroy -target=module.eks`), старый 1.28 штатными средствами уже не вернуть (EOL, только recreate на 1.35). Фактически «откат» = `destroy + apply` заново. Стейт — в S3 (`production/terraform.tfstate`), лок гарантирует DynamoDB-таблица.
- CD-apply (автоматический `terraform apply` из пайплайна) — только после S-118; сейчас gating ручной.
