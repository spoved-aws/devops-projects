{{/*
Expand the name of the chart.
*/}}
{{- define "two-tier-flask-app.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "two-tier-flask-app.fullname" -}}
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
{{- define "two-tier-flask-app.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "two-tier-flask-app.labels" -}}

helm.sh/chart: {{ include "two-tier-flask-app.chart" . }}

# Required Kubernetes recommended labels
app.kubernetes.io/name: {{ include "two-tier-flask-app.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}

# Enterprise extensions
app.kubernetes.io/component: {{ .Values.component | default "backend" }}
app.kubernetes.io/cluster: {{ .Values.cluster | default "k3s-synology" }}

environment: {{ .Values.environment | default "dev" }}
team: {{ .Values.team | default "platform" }}
tier: {{ .Values.tier | default "api" }}

{{- end }}

{{/*
Selector labels
*/}}
{{- define "two-tier-flask-app.selectorLabels" -}}
app.kubernetes.io/name: {{ include "two-tier-flask-app.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "two-tier-flask-app.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "two-tier-flask-app.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}


{{- define "two-tier-flask-app.selectorLabels.mysql" -}}
app.kubernetes.io/name: mysql
app.kubernetes.io/instance: {{ .Release.Name }}

# Enterprise extensions
component: database
tier: backend
part-of: {{ include "two-tier-flask-app.name" . }}
{{- end }}