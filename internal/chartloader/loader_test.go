package chartloader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverSuitesSorted(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, filepath.Join(root, "z_test.yaml"), "suite: z\n")
	writeFile(t, filepath.Join(root, "nested", "a_test.yaml"), "suite: a\n")
	writeFile(t, filepath.Join(root, "skip.yaml"), "suite: skip\n")

	suites, err := DiscoverSuites(root)
	if err != nil {
		t.Fatalf("discover suites: %v", err)
	}
	if len(suites) != 2 {
		t.Fatalf("expected 2 suites, got %d", len(suites))
	}
	if filepath.Base(suites[0]) != "a_test.yaml" || filepath.Base(suites[1]) != "z_test.yaml" {
		t.Fatalf("expected deterministic order, got %v", suites)
	}
}

func TestDiscoverChartsFindsNestedChartsSorted(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	chartA := filepath.Join(root, "charts", "common", "chart-a")
	chartB := filepath.Join(root, "charts", "foo", "chart-b")
	writeFile(t, filepath.Join(chartA, "Chart.yaml"), "apiVersion: v2\nname: chart-a\nversion: 0.1.0\n")
	writeFile(t, filepath.Join(chartB, "Chart.yaml"), "apiVersion: v2\nname: chart-b\nversion: 0.1.0\n")

	charts, err := DiscoverCharts(root)
	if err != nil {
		t.Fatalf("discover charts: %v", err)
	}
	if len(charts) != 2 {
		t.Fatalf("expected 2 charts, got %d", len(charts))
	}
	if charts[0] != chartA || charts[1] != chartB {
		t.Fatalf("expected sorted chart paths, got %v", charts)
	}
}

func TestLoadBundleMergesValuesWithDeterministicPrecedence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	chartDir := filepath.Join(root, "chart")
	testsDir := filepath.Join(root, "tests")
	valuesDir := filepath.Join(root, "values")
	if err := os.MkdirAll(chartDir, 0o755); err != nil {
		t.Fatalf("mkdir chart: %v", err)
	}
	if err := os.MkdirAll(testsDir, 0o755); err != nil {
		t.Fatalf("mkdir tests: %v", err)
	}
	if err := os.MkdirAll(valuesDir, 0o755); err != nil {
		t.Fatalf("mkdir values: %v", err)
	}

	writeFile(t, filepath.Join(chartDir, "Chart.yaml"), "apiVersion: v2\nname: demo\nversion: 0.1.0\n")
	writeFile(t, filepath.Join(chartDir, "values.yaml"), "image:\n  repository: nginx\n  tag: stable\nfeatureFlag: false\n")
	writeFile(t, filepath.Join(valuesDir, "staging.yaml"), "image:\n  tag: canary\nfeatureFlag: true\n")
	writeFile(t, filepath.Join(testsDir, "deployment_test.yaml"), `
suite: deployment
templates:
  - deployment.yaml
values:
  - ../values/staging.yaml
set:
  image.repository: myrepo/nginx
  featureFlag: true
`)

	bundle, err := LoadBundle(chartDir, testsDir)
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}

	if bundle.Chart.Name != "demo" {
		t.Fatalf("unexpected chart name: %s", bundle.Chart.Name)
	}
	if len(bundle.Suites) != 1 {
		t.Fatalf("expected 1 suite, got %d", len(bundle.Suites))
	}

	merged := bundle.Suites[0].MergedValues
	image := merged["image"].(map[string]any)
	if image["repository"] != "myrepo/nginx" {
		t.Fatalf("expected set value precedence for image.repository, got %v", image["repository"])
	}
	if image["tag"] != "canary" {
		t.Fatalf("expected values file to override chart values for image.tag, got %v", image["tag"])
	}
	if merged["featureFlag"] != true {
		t.Fatalf("expected set/values precedence for featureFlag, got %v", merged["featureFlag"])
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}
