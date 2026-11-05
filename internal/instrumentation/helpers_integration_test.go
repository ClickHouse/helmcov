package instrumentation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ClickHouse/helmcov/internal/chartloader"
)

func TestBranchHeavyHelpersGetLineHitsThroughChartLoaderPaths(t *testing.T) {
	t.Parallel()

	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for repoRoot != "" && repoRoot != "/" {
		if _, statErr := os.Stat(filepath.Join(repoRoot, "go.mod")); statErr == nil {
			break
		}
		repoRoot = filepath.Dir(repoRoot)
	}
	chartPath := filepath.Join(repoRoot, "examples/branch-heavy-chart")
	chartTemplates, err := chartloader.LoadTemplateFiles(chartPath)
	if err != nil {
		t.Fatalf("load templates: %v", err)
	}

	servicePath, err := chartloader.ResolveTemplatePath(chartPath, "templates/service.yaml")
	if err != nil {
		t.Fatalf("resolve service: %v", err)
	}
	content, err := os.ReadFile(servicePath)
	if err != nil {
		t.Fatalf("read service: %v", err)
	}

	exec := NewExecutor()
	trace, _, err := exec.RenderAndTrace(servicePath, string(content), map[string]any{
		"Values": map[string]any{
			"service": map[string]any{
				"type": "ClusterIP",
				"ports": []map[string]any{
					{"port": 80, "targetPort": 8080},
				},
			},
		},
		"Chart":   map[string]any{"Name": "branch-heavy"},
		"Release": map[string]any{"Name": "release-name"},
	}, chartTemplates)
	if err != nil {
		t.Fatalf("render trace: %v", err)
	}

	helperPath := ""
	for p := range chartTemplates {
		if filepath.Base(p) == "_helpers.tpl" {
			helperPath = p
			break
		}
	}

	hits := 0
	total := 0
	for k, v := range trace.Lines {
		if len(k) > len(helperPath) && k[:len(helperPath)] == helperPath {
			total++
			if v > 0 {
				hits++
			}
		}
	}
	t.Logf("helper path=%q hits=%d total=%d", helperPath, hits, total)
	if hits == 0 {
		t.Fatalf("expected helper line hits in single trace, branches=%v", trace.Branches)
	}
}

func TestRenderAndTraceDoesNotRegisterUnrelatedTemplateLines(t *testing.T) {
	t.Parallel()

	exec := NewExecutor()
	shared := map[string]string{
		"notes.txt":   "This chart was installed.\n",
		"helpers.tpl": `{{- define "demo.name" -}}demo{{- end -}}`,
	}
	main := `name: {{ include "demo.name" . }}`

	trace, _, err := exec.RenderAndTrace("configmap.yaml", main, map[string]any{}, shared)
	if err != nil {
		t.Fatalf("render trace: %v", err)
	}
	for key := range trace.Lines {
		if strings.HasPrefix(key, "notes.txt:") {
			t.Fatalf("expected unrelated template lines to be excluded, got %q", key)
		}
	}
}

func TestRenderAndTraceOnlyCountsLinesThatRender(t *testing.T) {
	t.Parallel()

	helpers := "{{- define \"demo.name\" -}}\n{{/*\ncomment line\n*/}}\nname: demo\n{{- end -}}"
	trace, _, err := NewExecutor().RenderAndTrace("t.yaml", "{{ include \"demo.name\" . }}\n", map[string]any{}, map[string]string{"helpers.tpl": helpers})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	// define, comment and end lines produce no output of their own.
	for _, line := range []string{"helpers.tpl:1", "helpers.tpl:2", "helpers.tpl:3", "helpers.tpl:4", "helpers.tpl:6"} {
		if _, ok := trace.Lines[line]; ok {
			t.Errorf("line %s should not be coverable", line)
		}
	}
	if trace.Lines["helpers.tpl:5"] == 0 {
		t.Errorf("helpers.tpl:5 should be covered; have %v", trace.Lines)
	}
}
