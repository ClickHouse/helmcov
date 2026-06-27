package chartloader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTemplateFilesLoadsYamlTplAndTxt(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	templatesDir := filepath.Join(root, "templates")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("mkdir templates: %v", err)
	}
	writeFile(t, filepath.Join(templatesDir, "_helpers.tpl"), `{{- define "demo.name" -}}demo{{- end -}}`)
	writeFile(t, filepath.Join(templatesDir, "configmap.yaml"), "kind: ConfigMap\n")
	writeFile(t, filepath.Join(templatesDir, "notes.txt"), "note\n")
	writeFile(t, filepath.Join(templatesDir, "README.md"), "ignored\n")

	files, err := LoadTemplateFiles(root)
	if err != nil {
		t.Fatalf("load template files: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 template files, got %d", len(files))
	}
}

func TestResolveTemplatePathSupportsHelmUnittestLayout(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	templatesDir := filepath.Join(root, "templates", "hyperdx")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("mkdir templates: %v", err)
	}
	writeFile(t, filepath.Join(templatesDir, "configmap.yaml"), "kind: ConfigMap\n")

	resolved, err := ResolveTemplatePath(root, "hyperdx/configmap.yaml")
	if err != nil {
		t.Fatalf("resolve template path: %v", err)
	}
	if resolved != filepath.Join(templatesDir, "configmap.yaml") {
		t.Fatalf("unexpected resolved path: %s", resolved)
	}

	resolved, err = ResolveTemplatePath(root, "templates/hyperdx/configmap.yaml")
	if err != nil {
		t.Fatalf("resolve template path with templates prefix: %v", err)
	}
	if resolved != filepath.Join(templatesDir, "configmap.yaml") {
		t.Fatalf("unexpected resolved path: %s", resolved)
	}
}

func TestHelmRenderValuesBuildsChartAndReleaseContext(t *testing.T) {
	t.Parallel()

	values := HelmRenderValues(RenderOptions{
		Chart: ChartMeta{
			Name:       "clickstack",
			Version:    "1.2.3",
			AppVersion: "4.5.6",
		},
		Values:       map[string]any{"enabled": true},
		ChartPath:    "/tmp/chart",
		TemplatePath: "/tmp/chart/templates/hyperdx/configmap.yaml",
		KubeVersion:  "1.29.0",
		Files:        ChartFiles{}.WithFile("config.txt", []byte("demo")),
	})

	chart, ok := values["Chart"].(map[string]any)
	if !ok {
		t.Fatalf("expected Chart map")
	}
	if chart["Name"] != "clickstack" || chart["AppVersion"] != "4.5.6" {
		t.Fatalf("unexpected chart context: %#v", chart)
	}

	release, ok := values["Release"].(map[string]any)
	if !ok {
		t.Fatalf("expected Release map")
	}
	if release["Name"] != "release-name" {
		t.Fatalf("unexpected release context: %#v", release)
	}

	scenario, ok := values["Values"].(map[string]any)
	if !ok || scenario["enabled"] != true {
		t.Fatalf("expected Values scenario map")
	}

	caps, ok := values["Capabilities"].(map[string]any)
	if !ok {
		t.Fatalf("expected Capabilities map")
	}
	kube := caps["KubeVersion"].(map[string]any)
	if kube["GitVersion"] != "v1.29.0" {
		t.Fatalf("expected kube version in capabilities, got %#v", kube["GitVersion"])
	}

	templateMeta, ok := values["Template"].(map[string]any)
	if !ok || templateMeta["Name"] != "hyperdx/configmap.yaml" || templateMeta["BasePath"] != "hyperdx" {
		t.Fatalf("expected template metadata, got %#v", templateMeta)
	}

	files, ok := values["Files"].(ChartFiles)
	if !ok {
		t.Fatalf("expected Files object")
	}
	if content, err := files.Get("config.txt"); err != nil || content != "demo" {
		t.Fatalf("expected chart file content, got %q err=%v", content, err)
	}
}
