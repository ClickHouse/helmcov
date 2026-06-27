package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
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
		name            string
		chart           string
		tests           string
		wantBranchBelow float64
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
			name:            "low-coverage",
			chart:           filepath.Join(repoRoot, "examples", "low-coverage-chart"),
			tests:           filepath.Join(repoRoot, "examples", "low-coverage-chart", "tests"),
			wantBranchBelow: 100.0,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			base := t.TempDir()
			if err := Run([]string{
				"--chart", tc.chart,
				"--tests", tc.tests,
				"--go-coverprofile", filepath.Join(base, "coverage.out"),
				"--cobertura-file", filepath.Join(base, "coverage.xml"),
			}, &out); err != nil {
				t.Fatalf("run failed: %v", err)
			}
			if out.Len() == 0 {
				t.Fatalf("expected output")
			}
			if _, err := os.Stat(filepath.Join(base, "coverage.out")); err != nil {
				t.Fatalf("expected go coverage output: %v", err)
			}
			if _, err := os.Stat(filepath.Join(base, "coverage.xml")); err != nil {
				t.Fatalf("expected cobertura output: %v", err)
			}
			if tc.wantBranchBelow > 0 {
				branchCoverage, err := parseBranchCoverage(out.String())
				if err != nil {
					t.Fatalf("parse branch coverage: %v", err)
				}
				if branchCoverage >= tc.wantBranchBelow {
					t.Fatalf("expected branch coverage below %.2f%%, got %.2f%%", tc.wantBranchBelow, branchCoverage)
				}
			}
		})
	}
}

func parseBranchCoverage(output string) (float64, error) {
	match := regexp.MustCompile(`branch-coverage=([0-9]+(?:\.[0-9]+)?)%`).FindStringSubmatch(output)
	if len(match) != 2 {
		return 0, fmt.Errorf("branch coverage summary not found in output")
	}
	return strconv.ParseFloat(match[1], 64)
}

func TestRunAgainstMonorepoExamples(t *testing.T) {
	t.Parallel()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("failed to locate current file")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	chartsRoot := filepath.Join(repoRoot, "examples", "monorepo", "charts")
	base := t.TempDir()

	var out bytes.Buffer
	if err := Run([]string{
		"--charts", chartsRoot,
		"--go-coverprofile", filepath.Join(base, "coverage.out"),
		"--cobertura-file", filepath.Join(base, "coverage.xml"),
	}, &out); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if out.Len() == 0 {
		t.Fatalf("expected output")
	}
	if _, err := os.Stat(filepath.Join(base, "coverage.out")); err != nil {
		t.Fatalf("expected go coverage output: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "coverage.xml")); err != nil {
		t.Fatalf("expected cobertura output: %v", err)
	}
}
