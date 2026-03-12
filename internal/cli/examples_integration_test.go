package cli

import (
	"bytes"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRunAgainstExamples(t *testing.T) {
	t.Parallel()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("failed to locate current file")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))

	cases := []struct {
		name  string
		chart string
		tests string
	}{
		{
			name:  "basic",
			chart: filepath.Join(repoRoot, "examples", "basic-chart"),
			tests: filepath.Join(repoRoot, "examples", "basic-chart", "tests"),
		},
		{
			name:  "branch-heavy",
			chart: filepath.Join(repoRoot, "examples", "branch-heavy-chart"),
			tests: filepath.Join(repoRoot, "examples", "branch-heavy-chart", "tests"),
		},
		{
			name:  "low-coverage",
			chart: filepath.Join(repoRoot, "examples", "low-coverage-chart"),
			tests: filepath.Join(repoRoot, "examples", "low-coverage-chart", "tests"),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			if err := Run([]string{"--chart", tc.chart, "--tests", tc.tests}, &out); err != nil {
				t.Fatalf("run failed: %v", err)
			}
			if out.Len() == 0 {
				t.Fatalf("expected output")
			}
		})
	}
}

func TestRunAgainstMonorepoExamples(t *testing.T) {
	t.Parallel()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("failed to locate current file")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	chartsRoot := filepath.Join(repoRoot, "examples", "monorepo", "charts")

	var out bytes.Buffer
	if err := Run([]string{"--charts", chartsRoot}, &out); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if out.Len() == 0 {
		t.Fatalf("expected output")
	}
}
