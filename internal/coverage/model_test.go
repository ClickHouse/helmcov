package coverage

import (
	"testing"

	"github.com/jordan-simonovski/helmcov/internal/instrumentation"
)

func TestFromTracesAggregatesHits(t *testing.T) {
	t.Parallel()

	traces := []instrumentation.Trace{
		{
			Lines: map[string]int{
				"templates/a.yaml:1": 1,
				"templates/a.yaml:2": 0,
			},
			Branches: map[string]int{
				"templates/a.yaml:if:true": 1,
			},
		},
		{
			Lines: map[string]int{
				"templates/a.yaml:1": 2,
			},
			Branches: map[string]int{
				"templates/a.yaml:if:false": 1,
			},
		},
	}

	report := FromTraces(traces)
	file := report.Files["templates/a.yaml"]
	if file.Lines[1] != 3 {
		t.Fatalf("expected line hit aggregation, got %d", file.Lines[1])
	}
	if file.Branches["if:true"] != 1 {
		t.Fatalf("expected if:true branch hit")
	}
	if file.Branches["if:false"] != 1 {
		t.Fatalf("expected if:false branch hit")
	}
}

func TestReportUncoveredHelpers(t *testing.T) {
	t.Parallel()

	report := Report{
		Files: map[string]FileCoverage{
			"templates/a.yaml": {
				Lines: map[int]int{
					1: 1,
					2: 0,
					3: 0,
				},
				Branches: map[string]int{
					"if:true":  0,
					"if:false": 1,
				},
			},
		},
	}

	uncoveredLines := report.UncoveredLines("templates/a.yaml")
	if len(uncoveredLines) != 2 || uncoveredLines[0] != 2 || uncoveredLines[1] != 3 {
		t.Fatalf("unexpected uncovered lines: %v", uncoveredLines)
	}

	uncoveredBranches := report.UncoveredBranches("templates/a.yaml")
	if len(uncoveredBranches) != 1 || uncoveredBranches[0] != "if:true" {
		t.Fatalf("unexpected uncovered branches: %v", uncoveredBranches)
	}
}
