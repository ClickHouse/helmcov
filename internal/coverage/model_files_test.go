package coverage

import (
	"reflect"
	"testing"

	"github.com/ClickHouse/helmcov/internal/instrumentation"
)

func TestReportPerFileMetrics(t *testing.T) {
	t.Parallel()

	report := Report{Files: map[string]FileCoverage{
		"b.yaml": {Lines: map[int]int{1: 2, 2: 0, 3: 1, 4: 5}, Branches: map[string]int{"1:if:true": 1, "1:if:false": 0, "3:with:empty": 0}},
		"a.yaml": {Lines: map[int]int{}, Branches: map[string]int{}},
	}}

	if got, want := report.SortedFiles(), []string{"a.yaml", "b.yaml"}; !reflect.DeepEqual(got, want) {
		t.Errorf("SortedFiles() = %v, want %v", got, want)
	}

	cases := []struct {
		file           string
		covered, total int
		lineRate       float64
		branchRate     float64
		uncoveredLines []int
	}{
		{file: "b.yaml", covered: 3, total: 4, lineRate: 0.75, branchRate: 1.0 / 3, uncoveredLines: []int{2}},
		{file: "a.yaml", covered: 0, total: 0, lineRate: 0, branchRate: 0, uncoveredLines: []int{}},
		{file: "missing.yaml", covered: 0, total: 0, lineRate: 0, branchRate: 0, uncoveredLines: nil},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()
			covered, total := report.FileCoveredLineCount(tc.file)
			if covered != tc.covered || total != tc.total {
				t.Errorf("FileCoveredLineCount() = %d/%d, want %d/%d", covered, total, tc.covered, tc.total)
			}
			if got := report.FileLineRate(tc.file); got != tc.lineRate {
				t.Errorf("FileLineRate() = %v, want %v", got, tc.lineRate)
			}
			if got := report.FileBranchRate(tc.file); got != tc.branchRate {
				t.Errorf("FileBranchRate() = %v, want %v", got, tc.branchRate)
			}
			if got := report.UncoveredLines(tc.file); !reflect.DeepEqual(got, tc.uncoveredLines) {
				t.Errorf("UncoveredLines() = %#v, want %#v", got, tc.uncoveredLines)
			}
		})
	}
}

func TestReportRatesWithNothingToCount(t *testing.T) {
	t.Parallel()

	reports := map[string]Report{
		"no files":               {Files: map[string]FileCoverage{}},
		"files without any keys": {Files: map[string]FileCoverage{"a.yaml": {Lines: map[int]int{}, Branches: map[string]int{}}}},
	}
	for name, report := range reports {
		if report.LineRate() != 0 || report.BranchRate() != 0 {
			t.Errorf("%s: rates = %v/%v, want 0/0", name, report.LineRate(), report.BranchRate())
		}
	}
	if got := (Report{}).UncoveredBranches("missing.yaml"); got != nil {
		t.Errorf("UncoveredBranches(missing) = %v, want nil", got)
	}
}

func TestFromTracesParsesKeys(t *testing.T) {
	t.Parallel()

	report := FromTraces([]instrumentation.Trace{{
		Lines: map[string]int{
			`C:\charts\t.yaml:7`: 1,
			"no-line-number":     1,
			"t.yaml:notanumber":  1,
		},
		Branches: map[string]int{
			"t.yaml:3:range:empty": 2,
			"legacy.yaml:if:true":  1,
			"nocolon":              1,
			"one:colon":            1,
		},
	}})

	want := map[string]FileCoverage{
		`C:\charts\t.yaml`: {Lines: map[int]int{7: 1}, Branches: map[string]int{}},
		"t.yaml":           {Lines: map[int]int{}, Branches: map[string]int{"3:range:empty": 2}},
		"legacy.yaml":      {Lines: map[int]int{}, Branches: map[string]int{"if:true": 1}},
	}
	if !reflect.DeepEqual(report.Files, want) {
		t.Errorf("FromTraces() files = %#v\nwant %#v", report.Files, want)
	}
}
