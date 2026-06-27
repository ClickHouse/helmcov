package instrumentation

import (
	"strings"
	"testing"
)

func TestRenderAndTraceRecordsIfRangeAndWithBranches(t *testing.T) {
	t.Parallel()

	exec := NewExecutor()
	template := `apiVersion: v1
kind: ConfigMap
{{ if .feature.enabled }}
metadata:
  name: enabled
{{ else }}
metadata:
  name: disabled
{{ end }}
items:
{{ range .items }}
  - {{ . }}
{{ else }}
  - none
{{ end }}
{{ with .labels }}
labels:
  app: {{ .app }}
{{ else }}
labels:
  app: none
{{ end }}
`

	trace, _, err := exec.RenderAndTrace("configmap.yaml", template, map[string]any{
		"feature": map[string]any{"enabled": true},
		"items":   []string{"a"},
		"labels":  map[string]any{"app": "demo"},
	}, nil)
	if err != nil {
		t.Fatalf("render trace: %v", err)
	}

	if trace.Branches["configmap.yaml:3:if:true"] == 0 {
		t.Fatalf("expected if true branch hit")
	}
	if trace.Branches["configmap.yaml:11:range:non-empty"] == 0 {
		t.Fatalf("expected range non-empty branch hit")
	}
	if trace.Branches["configmap.yaml:16:with:non-empty"] == 0 {
		t.Fatalf("expected with non-empty branch hit")
	}
}

func TestRenderAndTraceRecordsElseEdges(t *testing.T) {
	t.Parallel()

	exec := NewExecutor()
	template := `{{ if .enabled }}yes{{ else }}no{{ end }}
{{ range .items }}{{ . }}{{ else }}empty{{ end }}
{{ with .labels }}has{{ else }}none{{ end }}`

	trace, _, err := exec.RenderAndTrace("else.yaml", template, map[string]any{
		"enabled": false,
		"items":   []string{},
		"labels":  nil,
	}, nil)
	if err != nil {
		t.Fatalf("render trace: %v", err)
	}

	if trace.Branches["else.yaml:1:if:false"] == 0 {
		t.Fatalf("expected if false branch hit")
	}
	if trace.Branches["else.yaml:2:range:empty"] == 0 {
		t.Fatalf("expected range empty branch hit")
	}
	if trace.Branches["else.yaml:3:with:empty"] == 0 {
		t.Fatalf("expected with empty branch hit")
	}
}

func TestRenderAndTraceRegistersUnhitBranchEdges(t *testing.T) {
	t.Parallel()

	exec := NewExecutor()
	template := `{{ if eq .mode "prod" }}prod{{ else }}dev{{ end }}`

	trace, _, err := exec.RenderAndTrace("unhit.yaml", template, map[string]any{
		"mode": "dev",
	}, nil)
	if err != nil {
		t.Fatalf("render trace: %v", err)
	}

	if _, ok := trace.Branches["unhit.yaml:1:if:true"]; !ok {
		t.Fatalf("expected unhit true edge to be registered")
	}
	if trace.Branches["unhit.yaml:1:if:true"] != 0 {
		t.Fatalf("expected unhit true edge count to be 0")
	}
	if trace.Branches["unhit.yaml:1:if:false"] == 0 {
		t.Fatalf("expected false edge hit")
	}
}

func TestRenderAndTraceRecordsLineHits(t *testing.T) {
	t.Parallel()

	exec := NewExecutor()
	template := "kind: ConfigMap\nmetadata:\n  name: demo\n"

	trace, rendered, err := exec.RenderAndTrace("lines.yaml", template, map[string]any{}, nil)
	if err != nil {
		t.Fatalf("render trace: %v", err)
	}

	if rendered == "" {
		t.Fatalf("expected rendered output")
	}
	if trace.Lines["lines.yaml:1"] == 0 {
		t.Fatalf("expected line 1 hit")
	}
}

func TestRenderAndTraceKeepsUnexecutedElseLineAtZero(t *testing.T) {
	t.Parallel()

	exec := NewExecutor()
	template := `{{ if .enabled }}
yes
{{ else }}
no
{{ end }}
`

	trace, _, err := exec.RenderAndTrace("lines-branch.yaml", template, map[string]any{
		"enabled": true,
	}, nil)
	if err != nil {
		t.Fatalf("render trace: %v", err)
	}

	if trace.Lines["lines-branch.yaml:4"] != 0 {
		t.Fatalf("expected else line to remain uncovered, got %d", trace.Lines["lines-branch.yaml:4"])
	}
}

func TestRenderAndTraceSupportsInclude(t *testing.T) {
	t.Parallel()

	exec := NewExecutor()
	helpers := map[string]string{
		"_helpers.tpl": `{{- define "demo.fullname" -}}
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
		"Values": map[string]any{},
		"Chart":  map[string]any{"Name": "demo"},
		"Release": map[string]any{
			"Name": "release-name",
		},
	}

	trace, rendered, err := exec.RenderAndTrace("configmap.yaml", main, values, helpers)
	if err != nil {
		t.Fatalf("render trace: %v", err)
	}
	if !strings.Contains(rendered, "release-name-demo") {
		t.Fatalf("expected include output in render, got %q", rendered)
	}
	if trace.Branches["_helpers.tpl:2:if:false"] == 0 && trace.Branches["_helpers.tpl:4:if:true"] == 0 {
		t.Fatalf("expected helper if branch to be traced")
	}
}

func TestRenderAndTraceSupportsTplInValues(t *testing.T) {
	t.Parallel()

	exec := NewExecutor()
	helpers := map[string]string{
		"_helpers.tpl": `{{- define "demo.fullname" -}}release-name-demo{{- end -}}`,
	}
	main := `apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ tpl .Values.serviceAccountName . }}
`

	values := map[string]any{
		"Values": map[string]any{
			"serviceAccountName": `'{{ include "demo.fullname" . }}-sa'`,
		},
	}

	_, rendered, err := exec.RenderAndTrace("serviceaccount.yaml", main, values, helpers)
	if err != nil {
		t.Fatalf("render trace: %v", err)
	}
	if !strings.Contains(rendered, "release-name-demo-sa") {
		t.Fatalf("expected tpl to evaluate nested include, got %q", rendered)
	}
}

func TestRenderAndTraceEvaluatesIncludeInIfBranch(t *testing.T) {
	t.Parallel()

	exec := NewExecutor()
	helpers := map[string]string{
		"_helpers.tpl": `{{- define "demo.enabled" -}}{{ .Values.enabled }}{{- end -}}`,
	}
	main := `{{ if eq (include "demo.enabled" .) "true" }}yes{{ else }}no{{ end }}`

	values := map[string]any{
		"Values": map[string]any{"enabled": true},
	}

	trace, rendered, err := exec.RenderAndTrace("branch.yaml", main, values, helpers)
	if err != nil {
		t.Fatalf("render trace: %v", err)
	}
	if strings.TrimSpace(rendered) != "yes" {
		t.Fatalf("expected rendered yes, got %q", rendered)
	}
	if trace.Branches["branch.yaml:1:if:true"] == 0 {
		t.Fatalf("expected if true branch hit when include returns true")
	}
}

func TestRenderAndTraceSupportsSprigDefaultPipe(t *testing.T) {
	t.Parallel()

	exec := NewExecutor()
	main := `type: {{ .Values.service.type | default "ClusterIP" }}`

	_, rendered, err := exec.RenderAndTrace("service.yaml", main, map[string]any{
		"Values": map[string]any{
			"service": map[string]any{},
		},
	}, nil)
	if err != nil {
		t.Fatalf("render trace: %v", err)
	}
	if !strings.Contains(rendered, "ClusterIP") {
		t.Fatalf("expected default pipe output, got %q", rendered)
	}
}

func TestRenderAndTraceSupportsToYamlAndNindent(t *testing.T) {
	t.Parallel()

	exec := NewExecutor()
	main := `metadata:
{{- with .Values.labels }}
{{ toYaml . | nindent 2 }}
{{- end }}`

	_, rendered, err := exec.RenderAndTrace("labels.yaml", main, map[string]any{
		"Values": map[string]any{
			"labels": map[string]any{"app": "demo"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("render trace: %v", err)
	}
	if !strings.Contains(rendered, "app: demo") {
		t.Fatalf("expected yaml labels in output, got %q", rendered)
	}
}

func TestTplSourceNameIsStablePerContent(t *testing.T) {
	t.Parallel()

	first := tplSourceName("{{ if .Values.enabled }}yes{{ end }}")
	second := tplSourceName("{{ if .Values.enabled }}yes{{ end }}")
	third := tplSourceName("{{ if .Values.enabled }}no{{ end }}")

	if first != second {
		t.Fatalf("expected stable tpl source names, got %q and %q", first, second)
	}
	if first == third {
		t.Fatalf("expected different tpl source names for different content")
	}
	if !strings.HasPrefix(first, "tpl:") {
		t.Fatalf("expected tpl source prefix, got %q", first)
	}
}

func TestRenderAndTraceRecordsBranchesInsideTplContent(t *testing.T) {
	t.Parallel()

	exec := NewExecutor()
	embedded := "{{ if .Values.enabled }}enabled{{ else }}disabled{{ end }}"
	main := "{{ tpl .Values.embedded . }}"
	values := map[string]any{
		"Values": map[string]any{
			"enabled":  true,
			"embedded": embedded,
		},
	}

	trace, rendered, err := exec.RenderAndTrace("configmap.yaml", main, values, nil)
	if err != nil {
		t.Fatalf("render trace: %v", err)
	}
	if rendered != "enabled" {
		t.Fatalf("expected tpl render enabled, got %q", rendered)
	}

	sourceName := tplSourceName(embedded)
	if trace.Branches[sourceName+":1:if:true"] == 0 {
		t.Fatalf("expected tpl if true branch hit, branches=%v", trace.Branches)
	}
	if _, ok := trace.Branches[sourceName+":1:if:false"]; !ok {
		t.Fatalf("expected tpl if false branch registered")
	}
}

func TestRenderAndTraceRecordsTplBranchesFromToYamlValues(t *testing.T) {
	t.Parallel()

	exec := NewExecutor()
	spec := map[string]any{
		"replicas": "{{ if .Values.hpa.enabled }}2{{ else }}1{{ end }}",
	}
	embedded := toYAML(spec)
	main := "{{- tpl (toYaml .Values.spec) . -}}"
	values := map[string]any{
		"Values": map[string]any{
			"spec": spec,
			"hpa": map[string]any{
				"enabled": true,
			},
		},
	}

	trace, _, err := exec.RenderAndTrace("workload.yaml", main, values, nil)
	if err != nil {
		t.Fatalf("render trace: %v", err)
	}

	sourceName := tplSourceName(embedded)
	if trace.Branches[sourceName+":1:if:true"] == 0 {
		t.Fatalf("expected nested tpl branch hit in toYaml values, branches=%v", trace.Branches)
	}
}

func TestRenderAndTraceRegistersUnhitTplBranch(t *testing.T) {
	t.Parallel()

	exec := NewExecutor()
	embedded := "{{ if .Values.enabled }}yes{{ else }}no{{ end }}"
	main := "{{ tpl .Values.embedded . }}"
	values := map[string]any{
		"Values": map[string]any{
			"enabled":  false,
			"embedded": embedded,
		},
	}

	trace, _, err := exec.RenderAndTrace("configmap.yaml", main, values, nil)
	if err != nil {
		t.Fatalf("render trace: %v", err)
	}

	sourceName := tplSourceName(embedded)
	if trace.Branches[sourceName+":1:if:false"] == 0 {
		t.Fatalf("expected tpl if false branch hit")
	}
	if trace.Branches[sourceName+":1:if:true"] != 0 {
		t.Fatalf("expected tpl if true branch to remain unhit")
	}
}
