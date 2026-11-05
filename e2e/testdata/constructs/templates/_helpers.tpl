{{- define "constructs.fullname" -}}
{{- if .Values.nameOverride }}
{{- .Values.nameOverride | trunc 63 }}
{{- else }}
{{- printf "%s-%s" .Release.Name .Chart.Name | trunc 63 }}
{{- end }}
{{- end }}

{{- define "constructs.item" -}}
{{- if .enabled }}
{{ .name }}: "on"
{{- else }}
{{ .name }}: "off"
{{- end }}
{{- end }}

{{- define "constructs.unused" -}}
never: rendered
{{- end }}
