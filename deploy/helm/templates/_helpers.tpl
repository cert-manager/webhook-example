{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "gcore-webhook.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "gcore-webhook.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "gcore-webhook.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "gcore-webhook.labels" -}}
app.kubernetes.io/name: {{ include "gcore-webhook.name" . }}
helm.sh/chart: {{ include "gcore-webhook.chart" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
PKI
*/}}
{{- define "gcore-webhook.selfSignedIssuer" -}}
{{ printf "%s-selfsign" (include "gcore-webhook.fullname" .) }}
{{- end -}}

{{- define "gcore-webhook.rootCAIssuer" -}}
{{ printf "%s-ca" (include "gcore-webhook.fullname" .) }}
{{- end -}}

{{- define "gcore-webhook.rootCACertificate" -}}
{{ printf "%s-ca" (include "gcore-webhook.fullname" .) }}
{{- end -}}

{{- define "gcore-webhook.servingCertificate" -}}
{{ printf "%s-webhook-tls" (include "gcore-webhook.fullname" .) }}
{{- end -}}
