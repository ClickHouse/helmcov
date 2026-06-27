{{- define "chart-b.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "chart-b.fullname" -}}
{{- printf "%s-%s" .Release.Name (include "chart-b.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "chart-b.labels" -}}
app.kubernetes.io/name: {{ include "chart-b.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}
