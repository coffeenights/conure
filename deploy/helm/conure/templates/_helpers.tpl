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

{{- define "conure.api.selectorLabels" -}}
app.kubernetes.io/name: {{ include "conure.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: api-server
{{- end }}

{{- define "conure.api.labels" -}}
{{ include "conure.labels" . }}
app.kubernetes.io/component: api-server
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
