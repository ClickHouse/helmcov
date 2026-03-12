package instrumentation

import "testing"

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
	})
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
	})
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
	})
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

	trace, rendered, err := exec.RenderAndTrace("lines.yaml", template, map[string]any{})
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
	})
	if err != nil {
		t.Fatalf("render trace: %v", err)
	}

	if trace.Lines["lines-branch.yaml:4"] != 0 {
		t.Fatalf("expected else line to remain uncovered, got %d", trace.Lines["lines-branch.yaml:4"])
	}
}
