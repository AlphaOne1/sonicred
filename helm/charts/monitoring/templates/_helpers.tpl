{{- /*
SPDX-FileCopyrightText: 2026 The SonicRed contributors.
SPDX-License-Identifier: MPL-2.0
*/ -}}

{{- /*
Expand the name of the chart.
*/ -}}
{{- define "monitoring.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" | lower }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "monitoring.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{-     .Values.fullnameOverride | trunc 63 | trimSuffix "-" | lower -}}
{{- else -}}
{{-     $name := default .Chart.Name .Values.nameOverride -}}
{{-     if contains $name .Release.Name -}}
{{-         .Release.Name | trunc 63 | trimSuffix "-" | lower -}}
{{-     else -}}
{{-         printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" | lower -}}
{{-     end -}}
{{- end -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "monitoring.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "monitoring.labels" -}}
helm.sh/chart: {{ include "monitoring.chart" . }}
{{ include "monitoring.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "monitoring.selectorLabels" -}}
app.kubernetes.io/name: {{ include "monitoring.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Service name of a Victoria* single-node server subchart (metrics, logs, traces).
Mirrors the naming of "vm.plain.fullname" in the victoria-metrics-common chart.
Usage: include "monitoring.victoriaServer" (dict "ctx" . "alias" "metrics")
*/}}
{{- define "monitoring.victoriaServer" -}}
{{- $values := index .ctx.Values .alias | default dict -}}
{{- $server := $values.server | default dict -}}
{{- if $server.fullnameOverride -}}
{{-     $server.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{-     $fullname := $values.fullnameOverride -}}
{{-     if not $fullname -}}
{{-         $name := default .alias $values.nameOverride -}}
{{-         $relName := .ctx.Release.Name -}}
{{-         $fullname = ternary $relName (printf "%s-%s" $relName $name) (contains $name $relName) -}}
{{-     end -}}
{{-     printf "%s-server" ($fullname | trunc 63 | trimSuffix "-") | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{/*
Fails the rendering if no Grafana admin password is given.
*/}}
{{- define "monitoring.validateGrafanaAdmin" -}}
{{- $admin := .Values.grafana.admin | default dict -}}
{{- if not $admin.existingSecret -}}
{{-     $_ := required "monitoring.grafana.adminPassword must be set, e.g. with --set monitoring.grafana.adminPassword=<...>" .Values.grafana.adminPassword -}}
{{- end -}}
{{- end -}}
