{{/*
Expand the name of the chart.
*/}}
{{- define "stair-platform.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "stair-platform.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "stair-platform.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "stair-platform.labels" -}}
helm.sh/chart: {{ include "stair-platform.chart" . }}
{{ include "stair-platform.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "stair-platform.selectorLabels" -}}
app.kubernetes.io/name: {{ include "stair-platform.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
apiSelectorLabels — селектор ТОЛЬКО для api-подов.

INF-11 (forensic 2026-09-27): обычный selectorLabels содержит только
{name, instance}, а воркер добавляет к ним component: worker. Поэтому Service
и ServiceMonitor матчили и api, и worker. Воркер HTTP не слушает вовсе
(в cmd/worker нет net.Listen/ListenAndServe), значит ~1/3 трафика на ClusterIP
уходило в под без listener'а — connection refused. В helm upgrade --atomic это
проявляется как нестабильность нагрузки, а не как падение деплоя, поэтому
долго оставалось незамеченным.

Тот же дефект уже был найден и исправлен в pdb.yaml (там он выбивал обе
реплики api при дренаже узла), но в service.yaml/servicemonitor.yaml
пропущен. Вынесено в отдельный helper, чтобы «селектор api» был одним
определением, а не копипастой по шаблонам.
*/}}
{{- define "stair-platform.apiSelectorLabels" -}}
{{ include "stair-platform.selectorLabels" . }}
app.kubernetes.io/component: api
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "stair-platform.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "stair-platform.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
INF (2026-09-26): запрет неиммутабельного тега образа.

В Chart.AppVersion стоял "1.0.0", и при пустом image.tag дефолт был
иммутабельным, но `--set image.tag=latest` проходил молча: релиз с
`imagePullPolicy: IfNotPresent` зависал на старом образе или тянул новый
непредсказуемо. Теперь latest и пустой тег — ошибка рендера.
*/}}
{{- define "stair-platform.imageTag" -}}
{{- $tag := default .Chart.AppVersion .Values.image.tag -}}
{{- if or (eq $tag "latest") (eq $tag "") -}}
{{- fail "image.tag must be an immutable tag (git SHA or version), 'latest' is forbidden" -}}
{{- end -}}
{{- $tag -}}
{{- end -}}
