{{- define "mode-config.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "mode-config.fullname" -}}
{{- printf "%s-%s" .Release.Name (include "mode-config.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "mode-config.labels" -}}
app.kubernetes.io/name: {{ include "mode-config.name" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}
