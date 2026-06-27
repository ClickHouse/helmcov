{{- define "branch-heavy.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "branch-heavy.fullname" -}}
{{- printf "%s-%s" .Release.Name (include "branch-heavy.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "branch-heavy.labels" -}}
app.kubernetes.io/name: {{ include "branch-heavy.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}
