package reporters

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jordan-simonovski/helmcov/internal/coverage"
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
