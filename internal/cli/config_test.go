package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseConfigRequiresChartFlag(t *testing.T) {
	t.Parallel()

	_, err := ParseConfig([]string{"--tests", "tests"})
	if err == nil {
		t.Fatalf("expected error when chart/charts flags are missing")
	}
}

func TestParseConfigDefaultsTestsPathFromChart(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	chartDir := filepath.Join(base, "chart")
	testsDir := filepath.Join(chartDir, "tests")
	if err := os.MkdirAll(testsDir, 0o755); err != nil {
		t.Fatalf("create tests dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("apiVersion: v2\nname: demo\nversion: 0.1.0\n"), 0o644); err != nil {
		t.Fatalf("write chart: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testsDir, "demo_test.yaml"), []byte("suite: smoke\n"), 0o644); err != nil {
		t.Fatalf("write suite: %v", err)
	}

	cfg, err := ParseConfig([]string{"--chart", chartDir})
	if err != nil {
		t.Fatalf("expected parse success with default tests path: %v", err)
	}
	if cfg.TestsPath != testsDir {
		t.Fatalf("expected default tests path %s, got %s", testsDir, cfg.TestsPath)
	}
}

func TestParseConfigAcceptsChartsWithoutTestsFlag(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	chartsRoot := filepath.Join(base, "charts")
	chartA := filepath.Join(chartsRoot, "common", "chart-a")
	testsA := filepath.Join(chartA, "tests")
	if err := os.MkdirAll(testsA, 0o755); err != nil {
		t.Fatalf("mkdir tests: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chartA, "Chart.yaml"), []byte("apiVersion: v2\nname: chart-a\nversion: 0.1.0\n"), 0o644); err != nil {
		t.Fatalf("write chart: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testsA, "demo_test.yaml"), []byte("suite: smoke\n"), 0o644); err != nil {
		t.Fatalf("write suite: %v", err)
	}

	cfg, err := ParseConfig([]string{"--charts", chartsRoot})
	if err != nil {
		t.Fatalf("expected charts mode to parse: %v", err)
	}
	if cfg.ChartsRootPath != chartsRoot {
		t.Fatalf("unexpected charts root: %s", cfg.ChartsRootPath)
	}
}

func TestParseConfigRejectsChartAndChartsRootTogether(t *testing.T) {
	t.Parallel()

	_, err := ParseConfig([]string{"--chart", "a", "--tests", "b", "--charts", "c"})
	if err == nil {
		t.Fatalf("expected conflict error when both --chart and --charts are set")
	}
}

func TestParseConfigAcceptsDeprecatedChartsRootAlias(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	chartsRoot := filepath.Join(base, "charts")
	chartA := filepath.Join(chartsRoot, "common", "chart-a")
	testsA := filepath.Join(chartA, "tests")
	if err := os.MkdirAll(testsA, 0o755); err != nil {
		t.Fatalf("mkdir tests: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chartA, "Chart.yaml"), []byte("apiVersion: v2\nname: chart-a\nversion: 0.1.0\n"), 0o644); err != nil {
		t.Fatalf("write chart: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testsA, "demo_test.yaml"), []byte("suite: smoke\n"), 0o644); err != nil {
		t.Fatalf("write suite: %v", err)
	}

	cfg, err := ParseConfig([]string{"--charts-root", chartsRoot})
	if err != nil {
		t.Fatalf("expected charts-root alias to parse: %v", err)
	}
	if cfg.ChartsRootPath != chartsRoot {
		t.Fatalf("unexpected charts root: %s", cfg.ChartsRootPath)
	}
}

func TestParseConfigRejectsMissingChartYaml(t *testing.T) {
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
	testFile := filepath.Join(testsDir, "deployment_test.yaml")
	if err := os.WriteFile(testFile, []byte("suite: smoke\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	_, err := ParseConfig([]string{"--chart", chartDir, "--tests", testsDir})
	if err == nil {
		t.Fatalf("expected error when Chart.yaml is missing")
	}
}

func TestParseConfigRejectsEmptyTestsDirectory(t *testing.T) {
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

	chartFile := filepath.Join(chartDir, "Chart.yaml")
	if err := os.WriteFile(chartFile, []byte("apiVersion: v2\nname: demo\nversion: 0.1.0\n"), 0o644); err != nil {
		t.Fatalf("write chart: %v", err)
	}

	_, err := ParseConfig([]string{"--chart", chartDir, "--tests", testsDir})
	if err == nil {
		t.Fatalf("expected error when no suite files are present")
	}
}

func TestParseConfigAcceptsValidPathsAndFormats(t *testing.T) {
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

	cfg, err := ParseConfig([]string{
		"--chart", chartDir,
		"--tests", testsDir,
		"--format", "go",
		"--format", "cobertura",
		"--threshold", "75",
		"--max-scenarios", "12",
		"--go-coverprofile", filepath.Join(base, "custom.out"),
		"--cobertura-file", filepath.Join(base, "custom.xml"),
	})
	if err != nil {
		t.Fatalf("expected config to parse: %v", err)
	}

	if cfg.ChartPath != chartDir {
		t.Fatalf("unexpected chart path: %s", cfg.ChartPath)
	}
	if cfg.TestsPath != testsDir {
		t.Fatalf("unexpected tests path: %s", cfg.TestsPath)
	}
	if len(cfg.Formats) != 2 {
		t.Fatalf("expected 2 formats, got %d", len(cfg.Formats))
	}
	if cfg.Threshold != 75 {
		t.Fatalf("unexpected threshold: %v", cfg.Threshold)
	}
	if cfg.MaxScenarios != 12 {
		t.Fatalf("unexpected max scenarios: %d", cfg.MaxScenarios)
	}
	if cfg.GoCoverProfilePath != filepath.Join(base, "custom.out") {
		t.Fatalf("unexpected go cover path: %s", cfg.GoCoverProfilePath)
	}
	if cfg.CoberturaPath != filepath.Join(base, "custom.xml") {
		t.Fatalf("unexpected cobertura path: %s", cfg.CoberturaPath)
	}
}
