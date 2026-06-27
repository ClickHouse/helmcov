package instrumentation

import (
	"path/filepath"
	"testing"
)

func TestRenderAndTraceRecordsHelperLineHitsWithAbsolutePaths(t *testing.T) {
	t.Parallel()
	helperPath := filepath.Join("/charts/clickstack/templates", "_helpers.tpl")
	mainPath := filepath.Join("/charts/clickstack/templates", "configmap.yaml")
	helpers := map[string]string{
		helperPath: `{{- define "demo.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name .Chart.Name -}}
{{- end -}}
{{- end -}}`,
	}
	main := `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ include "demo.fullname" . }}
`
	values := map[string]any{
		"Values":  map[string]any{},
		"Chart":   map[string]any{"Name": "demo"},
		"Release": map[string]any{"Name": "release-name"},
	}
	exec := NewExecutor()
	trace, _, err := exec.RenderAndTrace(mainPath, main, values, helpers)
	if err != nil {
		t.Fatalf("render trace: %v", err)
	}
	hits := 0
	for k, v := range trace.Lines {
		if v > 0 && len(k) > len(helperPath) && k[:len(helperPath)] == helperPath {
			hits++
		}
	}
	if hits == 0 {
		t.Fatalf("expected helper line hits, lines=%v branches=%v", trace.Lines, trace.Branches)
	}
}
