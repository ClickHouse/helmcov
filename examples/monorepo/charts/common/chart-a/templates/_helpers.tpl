{{- define "chart-a.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "chart-a.fullname" -}}
{{- printf "%s-%s" .Release.Name (include "chart-a.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "chart-a.labels" -}}
app.kubernetes.io/name: {{ include "chart-a.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}
