#!/usr/bin/env bash
# STAIR PLATFORM — гейт дефектов Helm-чарта (INF-10/11/12, forensic 2026-09-27).
#
# Каждая проверка соответствует подтверждённому дефекту, а не общему
# «правильному» стилю. `helm lint` такие вещи не ловит: все три рендерились
# без ошибок и были применены.
#
#   INF-10  `toYaml X | default Y` при X=nil даёт `resources: null`,
#           потому что toYaml nil возвращает НЕПУСТУЮ строку "null",
#           и default на ней не срабатывает. Воркер уходил BestEffort без
#           limits. Ловим: рендерим и ищем `resources:\s*null`.
#
#   INF-11  Service и ServiceMonitor селектили и api, и worker (в worker-
#           подах есть component: worker, в селекторе его не было). Воркер
#           HTTP не слушает, поэтому часть трафика на ClusterIP уходила в
#           под без listener'а. Ловим: в рендере Service/servicemonitor
#           обязан быть component: api.
#
#   INF-12  OAuth-клиентский секрет попадал в ConfigMap открытым текстом,
#           тогда как secretsKey и Stripe-ключи лежали в Secret. Ловим:
#           в рендере ConfigMap не должно быть *_SECRET со значением.
#
# Usage:
#   scripts/check-helm-templates.sh
#   HELM=helm scripts/check-helm-templates.sh

set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
HELM="${HELM:-helm}"
command -v "$HELM" >/dev/null 2>&1 || HELM="$(command -v helm 2>/dev/null || true)"
if [[ -z "$HELM" || ! -x "$HELM" ]]; then
  for cand in /opt/homebrew/bin/helm /usr/local/bin/helm "$HOME/bin/helm"; do
    if [[ -x "$cand" ]]; then HELM="$cand"; break; fi
  done
fi
if [[ -z "$HELM" || ! -x "$HELM" ]]; then
  echo "ERROR: helm не найден (переменная HELM)." >&2
  exit 1
fi

CHART="$ROOT/deploy/helm/stair-platform"
# Рендерим с обязательным secretsKey (fail-fast S-105 в самом чарте) и с
# заполненным SSO, иначе ветка с секретом не попадёт в вывод.
KEY="$(printf 'ab%.0s' {1..32})"
OUT="$(mktemp)"
trap 'rm -f "$OUT"' EXIT

if ! "$HELM" template gate "$CHART" \
      --set secretsKey="$KEY" \
      --set sso.enabled=true \
      --set sso.issuerUrl=https://idp.example.com \
      --set sso.clientId=gate-client \
      --set sso.clientSecret=GATE_PLAINTEXT_SECRET \
      --set sso.redirectURL=https://app.example.com/callback \
      > "$OUT" 2>&1; then
  echo "ERROR: helm template не отрендерился:" >&2
  cat "$OUT" >&2
  exit 1
fi

fail=0

# ---- INF-10a: статически, само выражение в шаблоне ----
# Проверка рендера НЕ ЛОВИТ откат шаблона, если в values.yaml стоит
# `worker.resources: {}`: тогда `.Values.worker.resources` не nil, `toYaml` даёт
# `"{}"`, и `default` не срабатывает — рендер корректен, а выражение в шаблоне
# уже неверное. Такая комбинация опасна: values по умолчанию можно переписать
# (собственный values-файл отеля, оверлей), и дефект вернётся молча.
# Поэтому проверяем ОБЕ стороны — форму выражения и результат рендера.
BAD_EXPR="$(grep -rnE 'toYaml[[:space:]]+\.Values\.[A-Za-z0-9_.-]*resources[[:space:]]*\|[[:space:]]*default' \
  "$CHART/templates" 2>/dev/null || true)"
if [[ -n "$BAD_EXPR" ]]; then
  echo "FAIL INF-10: 'toYaml X | default Y' — default внутри скобок toYaml не работает:"
  echo "$BAD_EXPR" | sed 's/^/    /'
  echo "  Правильно: toYaml (.Values.X | default .Values.Y)"
  fail=1
else
  echo "ok   INF-10a: выражение toYaml(... | default ...) корректно"
fi

# ---- INF-10b: результат рендера ----
if grep -nE "^[[:space:]]*resources:[[:space:]]*null[[:space:]]*$" "$OUT" >/dev/null; then
  echo "FAIL INF-10: в рендере есть 'resources: null' — контейнер останется без limits."
  grep -nE "^[[:space:]]*resources:[[:space:]]*null[[:space:]]*$" "$OUT" | head -5
  echo "  Причина: 'toYaml .Values.worker.resources | default .Values.resources'."
  echo "  toYaml nil возвращает непустую строку 'null', и default не срабатывает."
  echo "  Правильно: toYaml (.Values.worker.resources | default .Values.resources)"
  fail=1
else
  echo "ok   INF-10: нет 'resources: null'"
fi

# ---- INF-11: селектор Service без component ----
svc_sel="$(awk '/^kind: Service$/{f=1} f&&/^---$/{f=0} f' "$OUT" | grep -A 6 '^  selector:')"
if ! grep -q "app.kubernetes.io/component: api" <<<"$svc_sel"; then
  echo "FAIL INF-11: селектор Service не ограничен component: api — матчит и worker."
  echo "$svc_sel" | sed 's/^/    /'
  echo "  Воркер HTTP не слушает, поэтому трафик на ClusterIP уходит в под без listener'а."
  fail=1
else
  echo "ok   INF-11: селектор Service ограничен component: api"
fi

# ---- INF-12: секрет в ConfigMap ----
cm="$(awk '/^kind: ConfigMap$/{f=1} f&&/^---$/{f=0} f' "$OUT")"
if grep -q "GATE_PLAINTEXT_SECRET" <<<"$cm"; then
  echo "FAIL INF-12: OAuth-секрет найден в ConfigMap — читается через 'kubectl get cm'."
  grep -n "GATE_PLAINTEXT_SECRET" <<<"$cm" | sed 's/^/    /'
  fail=1
else
  echo "ok   INF-12: OAuth-секрета нет в ConfigMap"
fi

# Секрет обязан присутствовать в Secret (иначе «защитили» бы, убрав доступ).
sec="$(awk '/^kind: Secret$/{f=1} f&&/^---$/{f=0} f' "$OUT")"
if ! grep -qE "^[[:space:]]*sso-client-secret:[[:space:]]*\"R0FURV9QTEFJTlRFWFRfU0VDUkVU" <<<"$sec"; then
  echo "FAIL INF-12: sso-client-secret отсутствует в Secret — переменная не дойдёт до контейнера."
  fail=1
else
  echo "ok   INF-12: sso-client-secret в Secret (base64)"
fi

# ---- Секреты вообще не должны попадать в ConfigMap ----
if grep -nE "^[[:space:]]+STAIR_[A-Z_]*(SECRET|PASSWORD|KEY|TOKEN)[A-Z_]*:[[:space:]]*\"[^\"$]" <<<"$cm" >/dev/null; then
  echo "FAIL: в ConfigMap найдена переменная с секретом в открытом виде:"
  grep -nE "^[[:space:]]+STAIR_[A-Z_]*(SECRET|PASSWORD|KEY|TOKEN)[A-Z_]*:" <<<"$cm" | sed 's/^/    /'
  fail=1
else
  echo "ok   в ConfigMap нет открытых секретов"
fi

if [[ $fail -ne 0 ]]; then
  echo
  echo "Гейт FAILED: дефекты Helm-чарта не устранены (см. INF-10/11/12 в REPORTS)."
  exit 1
fi

echo
echo "Гейт PASSED: дефекты INF-10/11/12 не вернулись."
