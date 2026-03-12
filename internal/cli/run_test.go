package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunWritesConfigSummary(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	chartDir := filepath.Join(base, "chart")
	testsDir := filepath.Join(base, "tests")
	if err := os.MkdirAll(chartDir, 0o755); err != nil {
		t.Fatalf("create chart dir: %v", err)
	}
	if err := os.MkdirAll(testsDir, 0o755); err != nil {
		t.Fatalf("create tests dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("apiVersion: v2\nname: demo\nversion: 0.1.0\n"), 0o644); err != nil {
		t.Fatalf("write chart: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testsDir, "demo_test.yaml"), []byte("suite: smoke\n"), 0o644); err != nil {
		t.Fatalf("write suite: %v", err)
	}

	var out bytes.Buffer
	err := Run([]string{"--chart", chartDir, "--tests", testsDir}, &out)
	if err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}

	if out.Len() == 0 {
		t.Fatalf("expected output summary")
	}
}

func TestRunWritesRequestedCoverageOutputs(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	chartDir := filepath.Join(base, "chart")
	testsDir := filepath.Join(base, "tests")
	templatesDir := filepath.Join(chartDir, "templates")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("create templates dir: %v", err)
	}
	if err := os.MkdirAll(testsDir, 0o755); err != nil {
		t.Fatalf("create tests dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("apiVersion: v2\nname: demo\nversion: 0.1.0\n"), 0o644); err != nil {
		t.Fatalf("write chart: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chartDir, "values.yaml"), []byte("feature:\n  enabled: true\n"), 0o644); err != nil {
		t.Fatalf("write values: %v", err)
	}
	if err := os.WriteFile(filepath.Join(templatesDir, "configmap.yaml"), []byte("{{ if .Values.feature.enabled }}enabled{{ else }}disabled{{ end }}\n"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testsDir, "demo_test.yaml"), []byte("suite: smoke\ntemplates:\n  - templates/configmap.yaml\n"), 0o644); err != nil {
		t.Fatalf("write suite: %v", err)
	}

	goOut := filepath.Join(base, "coverage.out")
	xmlOut := filepath.Join(base, "coverage.xml")

	var out bytes.Buffer
	err := Run([]string{
		"--chart", chartDir,
		"--tests", testsDir,
		"--format", "go",
		"--format", "cobertura",
		"--go-coverprofile", goOut,
		"--cobertura-file", xmlOut,
	}, &out)
	if err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}

	if _, err := os.Stat(goOut); err != nil {
		t.Fatalf("expected go coverprofile output file: %v", err)
	}
	if _, err := os.Stat(xmlOut); err != nil {
		t.Fatalf("expected cobertura output file: %v", err)
	}
}

func TestRunReportsMissingBranchCoverage(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	chartDir := filepath.Join(base, "chart")
	testsDir := filepath.Join(base, "tests")
	templatesDir := filepath.Join(chartDir, "templates")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("create templates dir: %v", err)
	}
	if err := os.MkdirAll(testsDir, 0o755); err != nil {
		t.Fatalf("create tests dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("apiVersion: v2\nname: lowcov\nversion: 0.1.0\n"), 0o644); err != nil {
		t.Fatalf("write chart: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chartDir, "values.yaml"), []byte("mode: dev\n"), 0o644); err != nil {
		t.Fatalf("write values: %v", err)
	}
	if err := os.WriteFile(filepath.Join(templatesDir, "mode.yaml"), []byte("{{ if eq .Values.mode \"prod\" }}prod{{ else }}dev{{ end }}\n"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testsDir, "mode_test.yaml"), []byte("suite: mode\ntemplates:\n  - templates/mode.yaml\n"), 0o644); err != nil {
		t.Fatalf("write suite: %v", err)
	}

	var out bytes.Buffer
	err := Run([]string{
		"--chart", chartDir,
		"--tests", testsDir,
		"--go-coverprofile", filepath.Join(base, "coverage.out"),
		"--cobertura-file", filepath.Join(base, "coverage.xml"),
	}, &out)
	if err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}

	if !strings.Contains(out.String(), "branch-coverage=50.00%") {
		t.Fatalf("expected branch coverage to report missing edge, got: %s", out.String())
	}
}

func TestRunVerboseShowsUncoveredLinesAndBranches(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	chartDir := filepath.Join(base, "chart")
	testsDir := filepath.Join(chartDir, "tests")
	templatesDir := filepath.Join(chartDir, "templates")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("create templates dir: %v", err)
	}
	if err := os.MkdirAll(testsDir, 0o755); err != nil {
		t.Fatalf("create tests dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("apiVersion: v2\nname: verbose\nversion: 0.1.0\n"), 0o644); err != nil {
		t.Fatalf("write chart: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chartDir, "values.yaml"), []byte("mode: dev\n"), 0o644); err != nil {
		t.Fatalf("write values: %v", err)
	}
	if err := os.WriteFile(filepath.Join(templatesDir, "mode.yaml"), []byte("{{ if eq .Values.mode \"prod\" }}\nprod\n{{ else }}\ndev\n{{ end }}\n"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	templateFile := filepath.Join(templatesDir, "mode.yaml")
	if err := os.WriteFile(filepath.Join(testsDir, "mode_test.yaml"), []byte("suite: mode\ntemplates:\n  - templates/mode.yaml\n"), 0o644); err != nil {
		t.Fatalf("write suite: %v", err)
	}

	var out bytes.Buffer
	err := Run([]string{
		"--chart", chartDir,
		"--go-coverprofile", filepath.Join(base, "coverage.out"),
		"--cobertura-file", filepath.Join(base, "coverage.xml"),
		"--verbose",
	}, &out)
	if err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "coverage details:") {
		t.Fatalf("expected verbose header, got: %s", output)
	}
	if !strings.Contains(output, "template: "+templateFile) {
		t.Fatalf("expected template section, got: %s", output)
	}
	if !strings.Contains(output, "uncovered lines:") {
		t.Fatalf("expected uncovered line block, got: %s", output)
	}
	if !strings.Contains(output, "uncovered branches:") {
		t.Fatalf("expected uncovered branch block, got: %s", output)
	}
	if !strings.Contains(output, "line-coverage:") {
		t.Fatalf("expected per-file line coverage, got: %s", output)
	}
	if !strings.Contains(output, templateFile+":2 ") {
		t.Fatalf("expected exact uncovered line reference, got: %s", output)
	}
	if !strings.Contains(output, templateFile+":1 ") || !strings.Contains(output, "(if:true)") {
		t.Fatalf("expected uncovered branch details, got: %s", output)
	}
}

func TestRunSupportsChartsRootMonorepoMode(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	chartsRoot := filepath.Join(base, "charts")
	chartA := filepath.Join(chartsRoot, "common", "chart-a")
	chartB := filepath.Join(chartsRoot, "foo", "chart-b")

	setupChart := func(chartPath string, chartName string) {
		t.Helper()
		templatesDir := filepath.Join(chartPath, "templates")
		testsDir := filepath.Join(chartPath, "tests")
		if err := os.MkdirAll(templatesDir, 0o755); err != nil {
			t.Fatalf("mkdir templates: %v", err)
		}
		if err := os.MkdirAll(testsDir, 0o755); err != nil {
			t.Fatalf("mkdir tests: %v", err)
		}
		if err := os.WriteFile(filepath.Join(chartPath, "Chart.yaml"), []byte("apiVersion: v2\nname: "+chartName+"\nversion: 0.1.0\n"), 0o644); err != nil {
			t.Fatalf("write chart: %v", err)
		}
		if err := os.WriteFile(filepath.Join(chartPath, "values.yaml"), []byte("enabled: true\n"), 0o644); err != nil {
			t.Fatalf("write values: %v", err)
		}
		if err := os.WriteFile(filepath.Join(templatesDir, "cm.yaml"), []byte("{{ if .Values.enabled }}yes{{ else }}no{{ end }}\n"), 0o644); err != nil {
			t.Fatalf("write template: %v", err)
		}
		if err := os.WriteFile(filepath.Join(testsDir, "suite_test.yaml"), []byte("suite: smoke\ntemplates:\n  - templates/cm.yaml\n"), 0o644); err != nil {
			t.Fatalf("write suite: %v", err)
		}
	}

	setupChart(chartA, "chart-a")
	setupChart(chartB, "chart-b")

	var out bytes.Buffer
	err := Run([]string{
		"--charts", chartsRoot,
		"--go-coverprofile", filepath.Join(base, "coverage.out"),
		"--cobertura-file", filepath.Join(base, "coverage.xml"),
	}, &out)
	if err != nil {
		t.Fatalf("unexpected run error in charts mode: %v", err)
	}
	if !strings.Contains(out.String(), "suites=2") {
		t.Fatalf("expected aggregate suite count for monorepo mode, got: %s", out.String())
	}
}
