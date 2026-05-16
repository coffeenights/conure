{{/*
Expand the name of the chart.
*/}}
{{- define "conure.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Fully qualified app name.
*/}}
{{- define "conure.fullname" -}}
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

{{- define "conure.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/* Common labels */}}
{{- define "conure.labels" -}}
helm.sh/chart: {{ include "conure.chart" . }}
app.kubernetes.io/name: {{ include "conure.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- with .Values.commonLabels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/* Controller naming */}}
{{- define "conure.controller.fullname" -}}
{{- printf "%s-controller" (include "conure.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "conure.controller.serviceAccountName" -}}
{{- if .Values.controller.serviceAccount.create }}
{{- default (include "conure.controller.fullname" .) .Values.controller.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.controller.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
selectorLabels MUST stay version-stable: these feed immutable fields
(Deployment/StatefulSet .spec.selector, StatefulSet .spec.volumeClaimTemplates).
Never add helm.sh/chart, app.kubernetes.io/version, or any per-release value
here, and never substitute "conure.*.labels" at those sites — doing so makes
`helm upgrade` fail with "updates to statefulset spec ... are forbidden".
*/}}
{{- define "conure.controller.selectorLabels" -}}
app.kubernetes.io/name: {{ include "conure.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: controller
{{- end }}

{{- define "conure.controller.labels" -}}
{{ include "conure.labels" . }}
app.kubernetes.io/component: controller
control-plane: controller-manager
{{- end }}

{{/* API server naming */}}
{{- define "conure.api.fullname" -}}
{{- printf "%s-api" (include "conure.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "conure.api.serviceAccountName" -}}
{{- if .Values.apiServer.serviceAccount.create }}
{{- default (include "conure.api.fullname" .) .Values.apiServer.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.apiServer.serviceAccount.name }}
{{- end }}
{{- end }}

{{/* selectorLabels MUST stay version-stable — see note on
conure.controller.selectorLabels above. */}}
{{- define "conure.api.selectorLabels" -}}
app.kubernetes.io/name: {{ include "conure.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: api-server
{{- end }}

{{- define "conure.api.labels" -}}
{{ include "conure.labels" . }}
app.kubernetes.io/component: api-server
{{- end }}

{{/*
Compute (and memoize) the admin password used by the create / rotate
hook Jobs and surfaced via NOTES.txt. The value is cached on
.Values.apiServer.adminUser._password so every `include` within a
single render returns the same string — both the Job env and NOTES.txt
must reference the same password.

Precedence:
  1. .Values.apiServer.adminUser.password (explicit override)
  2. randAlphaNum 16 (auto-generated, single-render only)

The password is intentionally NOT persisted to a Secret. Surface it via
NOTES once; if it's lost, rotate via `helm upgrade --set
apiServer.adminUser.rotate=true`.
*/}}
{{- define "conure.api.adminPassword" -}}
{{- $admin := .Values.apiServer.adminUser -}}
{{- if not (hasKey $admin "_password") -}}
  {{- if $admin.password -}}
    {{- $_ := set $admin "_password" $admin.password -}}
  {{- else -}}
    {{- $_ := set $admin "_password" (randAlphaNum 16) -}}
  {{- end -}}
{{- end -}}
{{- $admin._password -}}
{{- end }}

{{- define "conure.api.configMapName" -}}
{{ include "conure.api.fullname" . }}
{{- end }}

{{- define "conure.api.secretName" -}}
{{- if .Values.apiServer.secrets.existingSecret -}}
{{ .Values.apiServer.secrets.existingSecret }}
{{- else -}}
{{ include "conure.api.fullname" . }}
{{- end -}}
{{- end }}

{{/* MongoDB naming (bundled inline) */}}
{{- define "conure.mongodb.fullname" -}}
{{- printf "%s-mongodb" (include "conure.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/* selectorLabels MUST stay version-stable — see note on
conure.controller.selectorLabels above. The mongodb StatefulSet's
volumeClaimTemplates use this; any per-release label here breaks upgrades. */}}
{{- define "conure.mongodb.selectorLabels" -}}
app.kubernetes.io/name: {{ include "conure.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: mongodb
{{- end }}

{{- define "conure.mongodb.labels" -}}
{{ include "conure.labels" . }}
app.kubernetes.io/component: mongodb
{{- end }}

{{/*
Resolve the MongoDB connection URI for the API server.

Precedence:
  1. apiServer.secrets.mongodbUri (sensitive, explicit override)
  2. apiServer.config.mongodbUri (non-sensitive, explicit override)
  3. Auto-derived from the bundled inline mongodb when enabled
*/}}
{{- define "conure.api.mongodbUri" -}}
{{- if .Values.apiServer.secrets.mongodbUri -}}
{{ .Values.apiServer.secrets.mongodbUri }}
{{- else if .Values.apiServer.config.mongodbUri -}}
{{ .Values.apiServer.config.mongodbUri }}
{{- else if .Values.mongodb.enabled -}}
{{- $user := .Values.mongodb.auth.username -}}
{{- $pass := .Values.mongodb.auth.password -}}
{{- $db := .Values.mongodb.auth.database -}}
{{- $svc := include "conure.mongodb.fullname" . -}}
{{- $port := .Values.mongodb.service.port | int -}}
{{- printf "mongodb://%s:%s@%s:%d/%s?authSource=%s" $user $pass $svc $port $db $db -}}
{{- else -}}
{{- fail "MongoDB URI required: set apiServer.config.mongodbUri, apiServer.secrets.mongodbUri, or mongodb.enabled=true" -}}
{{- end -}}
{{- end }}
