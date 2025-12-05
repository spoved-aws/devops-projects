{{/* Generic app name */}}
{{- define "golden.name" -}}
{{ .Chart.Name }}
{{- end }}

{{/* Full release name */}}
{{- define "golden.fullname" -}}
{{ .Release.Name }}-{{ include "golden.name" . }}
{{- end }}

{{/* Common labels */}}
{{- define "golden.labels" -}}
app.kubernetes.io/name: {{ include "golden.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}