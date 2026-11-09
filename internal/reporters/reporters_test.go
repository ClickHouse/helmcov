package reporters

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ClickHouse/helmcov/internal/coverage"
)

func TestWriteGoCoverProfile(t *testing.T) {
	t.Parallel()

	report := coverage.Report{
		Files: map[string]coverage.FileCoverage{
			"templates/configmap.yaml": {
				Lines: map[int]int{
					1: 1,
					2: 0,
					3: 2,
				},
				Branches: map[string]int{
					"if:true":  1,
					"if:false": 0,
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := WriteGoCoverProfile(report, &buf); err != nil {
		t.Fatalf("write go coverprofile: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "mode: count") {
		t.Fatalf("missing mode header: %s", got)
	}
	if !strings.Contains(got, "templates/configmap.yaml:1.1,1.1 1 1") {
		t.Fatalf("missing expected line coverage entry: %s", got)
	}
}

func TestWriteCoberturaXML(t *testing.T) {
	t.Parallel()

	report := coverage.Report{
		Files: map[string]coverage.FileCoverage{
			"templates/configmap.yaml": {
				Lines: map[int]int{
					1: 1,
					2: 0,
				},
				Branches: map[string]int{
					"if:true":  1,
					"if:false": 1,
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := WriteCoberturaXML(report, &buf); err != nil {
		t.Fatalf("write cobertura xml: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "<coverage") {
		t.Fatalf("missing coverage root: %s", got)
	}
	if !strings.Contains(got, `filename="templates/configmap.yaml"`) {
		t.Fatalf("missing class filename: %s", got)
	}
	if !strings.Contains(got, `line number="1"`) {
		t.Fatalf("missing line entry: %s", got)
	}
}

func TestWriteMarkdownIncludesSummaryAndMarker(t *testing.T) {
	t.Parallel()

	report := coverage.Report{
		Files: map[string]coverage.FileCoverage{
			"templates/configmap.yaml": {
				Lines: map[int]int{
					1: 1,
					2: 0,
				},
				Branches: map[string]int{
					"1:if:true":  1,
					"1:if:false": 0,
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := WriteMarkdown(report, &buf, MarkdownOptions{
		Threshold:       70,
		CommentMarker:   "helmcov-comment",
		ShowFileSummary: true,
	}); err != nil {
		t.Fatalf("write markdown: %v", err)
	}

	got := buf.String()
	for _, want := range []string{
		"<!-- helmcov-comment -->",
		"## Helm template coverage",
		"| Line coverage | 50.00% |",
		"| Branch coverage | 50.00% |",
		"| Line threshold | 70% (not met) |",
		"| templates/configmap.yaml | 50.00% | 50.00% |",
		"<summary>Uncovered details</summary>",
		"**Uncovered lines:** 2",
		"**Uncovered branches:** 1:if:false",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in markdown output:\n%s", want, got)
		}
	}
}

func TestWriteMarkdownHidesFileSummaryByDefault(t *testing.T) {
	t.Parallel()

	report := coverage.Report{
		Files: map[string]coverage.FileCoverage{
			"templates/configmap.yaml": {
				Lines: map[int]int{1: 1},
			},
		},
	}

	var buf bytes.Buffer
	if err := WriteMarkdown(report, &buf, MarkdownOptions{}); err != nil {
		t.Fatalf("write markdown: %v", err)
	}

	got := buf.String()
	if strings.Contains(got, "### Per-file summary") {
		t.Fatalf("expected per-file summary to be hidden by default:\n%s", got)
	}
	if !strings.Contains(got, "#### templates/configmap.yaml") {
		t.Fatalf("expected uncovered details for chart templates:\n%s", got)
	}
}

func TestWriteMarkdownExcludesTplSourcesByDefault(t *testing.T) {
	t.Parallel()

	report := coverage.Report{
		Files: map[string]coverage.FileCoverage{
			"templates/deployment.yaml": {
				Lines: map[int]int{1: 1},
			},
			"tpl:135df8fd": {
				Lines: map[int]int{1: 0},
			},
		},
	}

	var buf bytes.Buffer
	if err := WriteMarkdown(report, &buf, MarkdownOptions{ShowFileSummary: true}); err != nil {
		t.Fatalf("write markdown: %v", err)
	}

	got := buf.String()
	if strings.Contains(got, "tpl:135df8fd") {
		t.Fatalf("expected tpl sources to be excluded by default:\n%s", got)
	}
	if !strings.Contains(got, "templates/deployment.yaml") {
		t.Fatalf("expected chart templates to remain:\n%s", got)
	}
}

func TestWriteMarkdownReportsThresholdMet(t *testing.T) {
	t.Parallel()

	report := coverage.Report{
		Files: map[string]coverage.FileCoverage{
			"templates/configmap.yaml": {
				Lines: map[int]int{1: 1},
			},
		},
	}

	var buf bytes.Buffer
	if err := WriteMarkdown(report, &buf, MarkdownOptions{Threshold: 50}); err != nil {
		t.Fatalf("write markdown: %v", err)
	}

	if !strings.Contains(buf.String(), "| Line threshold | 50% (met) |") {
		t.Fatalf("expected threshold met marker, got:\n%s", buf.String())
	}
}

func TestWriteMarkdownShowsTestsOnlyColumn(t *testing.T) {
	t.Parallel()

	all := coverage.Report{Files: map[string]coverage.FileCoverage{
		"t.yaml": {Lines: map[int]int{1: 1, 2: 1, 3: 1, 4: 0}, Branches: map[string]int{"1:if:true": 1, "1:if:false": 1}},
	}}
	tests := coverage.Report{Files: map[string]coverage.FileCoverage{
		"t.yaml": {Lines: map[int]int{1: 1, 2: 0, 3: 0, 4: 0}, Branches: map[string]int{"1:if:true": 0, "1:if:false": 1}},
	}}
	var buf bytes.Buffer
	if err := WriteMarkdown(all, &buf, MarkdownOptions{Tests: &tests, Threshold: 70}); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	for _, want := range []string{
		"| Metric | Value | Tests only |\n|--------|------:|-----------:|\n",
		"| Line coverage | 75.00% | 25.00% |\n",
		"| Branch coverage | 100.00% | 50.00% |\n",
		"| Line threshold | 70% (met) |\n",
		"Tests only counts what helm-unittest tests render.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	buf.Reset()
	if err := WriteMarkdown(all, &buf, MarkdownOptions{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "Tests only") {
		t.Errorf("tests-only column without a tests report:\n%s", buf.String())
	}
}
