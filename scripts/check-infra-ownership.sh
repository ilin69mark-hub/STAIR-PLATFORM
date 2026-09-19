#!/usr/bin/env bash
set -euo pipefail

# S2-2 gate: один владелец у каждого ресурса.
#  - terraform НЕ объявляет helm-ресурсы/провайдера (приложение живёт в CD)
#  - CD/helm НЕ дублирует инфраструктурные ресурсы terraform
# Падает с ненулевым кодом, если правило нарушено.

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TF_DIR="$ROOT/deploy/terraform"
HELM_DIR="$ROOT/deploy/helm"
CD="$ROOT/.github/workflows/cd.yml"

fail=0

# 1. terraform без helm_release / helm-провайдера (комментарии не считаем)
if grep -rn -E "^\s*resource\s+\"helm_release\"|^\s*helm\s*=|helm_release\s*{" \
     "$TF_DIR" 2>/dev/null | grep -v -E ":\s*#"; then
  echo "ERROR (S2-2): terraform управляет helm-ресурсом — приложение обязано жить в CD (ci/cd.yml)." >&2
  echo "Удалите helm-блок из deploy/terraform; helm-чарт разворачивает только CD." >&2
  fail=1
else
  echo "OK (S2-2): terraform не объявляет helm-ресурсов."
fi

# 2. CD НЕ дублирует terraform-инварианты: образ берётся только из APP_VERSION
if grep -qE "image.tag=latest|image.tag.*latest" "$CD"; then
  echo "ERROR (S2-3): CD фиксирует image.tag=latest — образ должен быть задан APP_VERSION (sha)." >&2
  fail=1
else
  echo "OK (S2-3): CD не закрепляет :latest для релиза."
fi

# 3. helm-чарт не содержит terraform/backends (напр., провайдеры)
if grep -rnE "terraform\s*\{" "$HELM_DIR" 2>/dev/null; then
  echo "ERROR (S2-2): helm-чарт содержит terraform-блок." >&2
  fail=1
else
  echo "OK (S2-2): helm-чарт не содержит terraform-кода."
fi

exit "$fail"