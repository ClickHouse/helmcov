package instrumentation

import (
	"strings"
	"testing"

	"github.com/jordan-simonovski/helmcov/internal/chartloader"
)

func TestFromYAMLReturnsMapOrErrorKey(t *testing.T) {
	t.Parallel()

	result := fromYAML("enabled: true\n")
	if result["enabled"] != true {
		t.Fatalf("expected parsed yaml value, got %#v", result)
	}

	invalid := fromYAML(":\n- bad")
	if _, ok := invalid["Error"]; !ok {
		t.Fatalf("expected Error key for invalid yaml, got %#v", invalid)
	}
}

func TestFromJSONAndToJSONRoundTrip(t *testing.T) {
	t.Parallel()

	payload := toJSON(map[string]any{"mode": "prod"})
	if payload == "" {
		t.Fatalf("expected json output")
	}

	result := fromJSON(payload)
	if result["mode"] != "prod" {
		t.Fatalf("expected parsed json value, got %#v", result)
	}
}

func TestFromYAMLArrayAndJSONArray(t *testing.T) {
	t.Parallel()

	yamlItems := fromYAMLArray("- a\n- b\n")
	if len(yamlItems) != 2 {
		t.Fatalf("expected 2 yaml array items, got %#v", yamlItems)
	}

	jsonItems := fromJSONArray(`["a","b"]`)
	if len(jsonItems) != 2 {
		t.Fatalf("expected 2 json array items, got %#v", jsonItems)
	}
}

func TestToYAMLPrettyIndentsOutput(t *testing.T) {
	t.Parallel()

	output := toYAMLPretty(map[string]any{
		"labels": map[string]any{"app": "demo"},
	})
	if !strings.Contains(output, "  app: demo") {
		t.Fatalf("expected indented yaml, got %q", output)
	}
}

func TestRenderAndTraceSupportsFromYamlInTemplate(t *testing.T) {
	t.Parallel()

	exec := NewExecutor()
	main := `{{ $cfg := .Values.config | fromYaml }}{{ if $cfg.enabled }}yes{{ else }}no{{ end }}`
	values := map[string]any{
		"Values": map[string]any{
			"config": "enabled: true\n",
		},
	}

	_, rendered, err := exec.RenderAndTrace("config.yaml", main, values, nil)
	if err != nil {
		t.Fatalf("render trace: %v", err)
	}
	if strings.TrimSpace(rendered) != "yes" {
		t.Fatalf("expected rendered yes, got %q", rendered)
	}
}

func TestRenderAndTraceSupportsCapabilitiesAPIVersionGate(t *testing.T) {
	t.Parallel()

	exec := NewExecutor()
	main := `{{ if .Capabilities.APIVersions.Has "batch/v1" }}batch{{ else }}none{{ end }}`
	values := map[string]any{
		"Capabilities": map[string]any{
			"APIVersions": chartloader.APIVersionSet{"batch/v1"},
		},
	}

	_, rendered, err := exec.RenderAndTrace("gate.yaml", main, values, nil)
	if err != nil {
		t.Fatalf("render trace: %v", err)
	}
	if strings.TrimSpace(rendered) != "batch" {
		t.Fatalf("expected batch, got %q", rendered)
	}
}

func TestRenderAndTraceSupportsFilesGet(t *testing.T) {
	t.Parallel()

	exec := NewExecutor()
	main := `{{ .Files.Get "config.txt" }}`
	values := map[string]any{
		"Files": chartloader.ChartFiles{}.WithFile("config.txt", []byte("enabled")),
	}

	_, rendered, err := exec.RenderAndTrace("files.yaml", main, values, nil)
	if err != nil {
		t.Fatalf("render trace: %v", err)
	}
	if strings.TrimSpace(rendered) != "enabled" {
		t.Fatalf("expected file content, got %q", rendered)
	}
}

func TestRenderAndTraceSupportsTemplateContext(t *testing.T) {
	t.Parallel()

	exec := NewExecutor()
	main := `{{ .Template.Name }}:{{ .Template.BasePath }}`
	values := map[string]any{
		"Template": map[string]any{
			"Name":     "hyperdx/configmap.yaml",
			"BasePath": "hyperdx",
		},
	}

	_, rendered, err := exec.RenderAndTrace("meta.yaml", main, values, nil)
	if err != nil {
		t.Fatalf("render trace: %v", err)
	}
	if rendered != "hyperdx/configmap.yaml:hyperdx" {
		t.Fatalf("expected template metadata, got %q", rendered)
	}
}

func TestRenderAndTraceContinuesOnFailInLintMode(t *testing.T) {
	t.Parallel()

	exec := NewExecutor()
	template := `{{ if .Values.enabled }}{{ fail "invalid values" }}after-fail{{ end }}`
	values := map[string]any{
		"Values": map[string]any{"enabled": true},
	}

	trace, rendered, err := exec.RenderAndTrace("ingress.yaml", template, values, nil)
	if err != nil {
		t.Fatalf("render trace: %v", err)
	}
	if !strings.Contains(rendered, "after-fail") {
		t.Fatalf("expected render to continue past fail, got %q", rendered)
	}
	if trace.Lines["ingress.yaml:1"] == 0 {
		t.Fatalf("expected fail line to be traced")
	}
}
