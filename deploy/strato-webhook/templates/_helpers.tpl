{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "strato-webhook.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "strato-webhook.fullname" -}}
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
{{- define "strato-webhook.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "strato-webhook.selfSignedIssuer" -}}
{{ printf "%s-selfsign" (include "strato-webhook.fullname" .) }}
{{- end -}}

{{- define "strato-webhook.rootCAIssuer" -}}
{{ printf "%s-ca" (include "strato-webhook.fullname" .) }}
{{- end -}}

{{- define "strato-webhook.rootCACertificate" -}}
{{ printf "%s-ca" (include "strato-webhook.fullname" .) }}
{{- end -}}

{{- define "strato-webhook.servingCertificate" -}}
{{ printf "%s-webhook-tls" (include "strato-webhook.fullname" .) }}
{{- end -}}

{{- define "strato-webhook.secretName" -}}
{{- default (include "strato-webhook.fullname" .) (.Values.token.existingSecretName) -}}
{{- end -}}