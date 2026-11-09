package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// halfCoveredChart has one template where 2 of 4 coverable lines and 1 of 2
// branches render with the default values.
func halfCoveredChart(t *testing.T) (chart, tests string) {
	t.Helper()
	base := t.TempDir()
	chart = filepath.Join(base, "chart")
	tests = filepath.Join(chart, "tests")
	writeTestFile(t, filepath.Join(chart, "Chart.yaml"), "apiVersion: v2\nname: demo\nversion: 0.1.0\n")
	writeTestFile(t, filepath.Join(chart, "values.yaml"), "x: false\n")
	writeTestFile(t, filepath.Join(chart, "templates", "t.yaml"), "a: 1\n{{- if .Values.x }}\nb: 2\nc: 3\n{{- end }}\n")
	writeTestFile(t, filepath.Join(tests, "t_test.yaml"), "suite: t\ntemplates:\n  - templates/t.yaml\n")
	return chart, tests
}

func runHalfCovered(t *testing.T, extra ...string) (string, error) {
	t.Helper()
	chart, tests := halfCoveredChart(t)
	out := t.TempDir()
	args := append([]string{
		"--chart", chart, "--tests", tests, "--max-scenarios", "1",
		"--go-coverprofile", filepath.Join(out, "c.out"),
		"--cobertura-file", filepath.Join(out, "c.xml"),
	}, extra...)
	var stdout bytes.Buffer
	err := Run(args, &stdout)
	return stdout.String(), err
}

func TestRunThresholdGate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		threshold string
		wantErr   bool
	}{
		{threshold: "0", wantErr: false},
		{threshold: "50", wantErr: false},
		{threshold: "50.5", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.threshold, func(t *testing.T) {
			t.Parallel()
			stdout, err := runHalfCovered(t, "--threshold", tc.threshold)
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), "coverage 50.00 is below threshold 50.50") {
					t.Fatalf("err = %v, want the below-threshold error", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(stdout, "line-coverage=50.00% branch-coverage=50.00%") {
				t.Errorf("summary = %q", stdout)
			}
		})
	}
}

func TestRunVerboseFlag(t *testing.T) {
	t.Parallel()

	quiet, err := runHalfCovered(t)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(quiet, "coverage details") {
		t.Errorf("details printed without --verbose:\n%s", quiet)
	}
}

func TestRunVerboseListsEveryUncoveredLineAndBranch(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	chart := filepath.Join(base, "chart")
	writeTestFile(t, filepath.Join(chart, "Chart.yaml"), "apiVersion: v2\nname: demo\nversion: 0.1.0\n")
	writeTestFile(t, filepath.Join(chart, "templates", "full.yaml"), "z: 1\n")
	writeTestFile(t, filepath.Join(chart, "templates", "two.yaml"), "{{- if .Values.x }}\nb: 2\n{{- end }}\n{{- if .Values.y }}\nc: 3\n{{- end }}\n")
	writeTestFile(t, filepath.Join(chart, "tests", "t_test.yaml"), "suite: t\ntemplates:\n  - templates/full.yaml\n  - templates/two.yaml\n")

	var stdout bytes.Buffer
	err := Run([]string{
		"--chart", chart, "--tests", filepath.Join(chart, "tests"), "--max-scenarios", "1", "--verbose",
		"--go-coverprofile", filepath.Join(base, "c.out"), "--cobertura-file", filepath.Join(base, "c.xml"),
	}, &stdout)
	if err != nil {
		t.Fatal(err)
	}

	full := filepath.Join(chart, "templates", "full.yaml")
	two := filepath.Join(chart, "templates", "two.yaml")
	want := fmt.Sprintf(`coverage details:
template: %[1]s
  line-coverage: 100.00%% (1/1)
  uncovered lines:
    - none
  uncovered branches:
    - none
template: %[2]s
  line-coverage: 50.00%% (2/4)
  uncovered lines:
    - %[2]s:2 "b: 2"
    - %[2]s:5 "c: 3"
  uncovered branches:
    - %[2]s:1 "{{- if .Values.x }}" (if:true)
    - %[2]s:4 "{{- if .Values.y }}" (if:true)
`, full, two)
	_, details, _ := strings.Cut(stdout.String(), "\n")
	if details != want {
		t.Errorf("verbose output =\n%s\nwant\n%s", details, want)
	}
}

func TestRunReportsUnwritableOutputs(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"write go coverprofile": {"--format", "go", "--go-coverprofile", "BLOCKED/c.out"},
		"write cobertura XML":   {"--format", "cobertura", "--cobertura-file", "BLOCKED/c.xml"},
		"write markdown":        {"--format", "markdown", "--markdown-file", "BLOCKED/c.md"},
	}
	for want, flags := range cases {
		t.Run(want, func(t *testing.T) {
			t.Parallel()
			// A regular file where a directory is needed makes the write fail.
			blocker := filepath.Join(t.TempDir(), "file")
			writeTestFile(t, blocker, "")
			for i, flag := range flags {
				flags[i] = strings.Replace(flag, "BLOCKED", blocker, 1)
			}
			_, err := runHalfCovered(t, flags...)
			if err == nil || !strings.HasPrefix(err.Error(), want) {
				t.Fatalf("err = %v, want prefix %q", err, want)
			}
		})
	}
}

func TestParseConfigBounds(t *testing.T) {
	t.Parallel()

	cases := []struct {
		flags   []string
		wantErr string
	}{
		{flags: []string{"--threshold", "-1"}, wantErr: "threshold must be between 0 and 100"},
		{flags: []string{"--threshold", "100.5"}, wantErr: "threshold must be between 0 and 100"},
		{flags: []string{"--threshold", "0"}},
		{flags: []string{"--threshold", "100"}},
		{flags: []string{"--max-scenarios", "0"}, wantErr: "max-scenarios must be > 0"},
		{flags: []string{"--max-scenarios", "1"}},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.flags, " "), func(t *testing.T) {
			t.Parallel()
			chart, tests := halfCoveredChart(t)
			_, err := ParseConfig(append([]string{"--chart", chart, "--tests", tests}, tc.flags...))
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestParseConfigRejectsChartsRootWithoutCharts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "nested", "values.yaml"), "x: 1\n")
	if _, err := ParseConfig([]string{"--charts", root}); err == nil {
		t.Fatal("want an error for a charts root with no Chart.yaml")
	}
}

func TestLabelsPreferChartsRoot(t *testing.T) {
	t.Parallel()

	single := Config{ChartPath: "chart", TestsPath: "chart/tests"}
	if targetLabel(single) != "chart" || testsLabel(single) != "chart/tests" {
		t.Errorf("single chart labels = %q, %q", targetLabel(single), testsLabel(single))
	}
	monorepo := Config{ChartsRootPath: "charts", ChartPath: "ignored"}
	if targetLabel(monorepo) != "charts" || testsLabel(monorepo) != "<auto: chart/tests>" {
		t.Errorf("charts root labels = %q, %q", targetLabel(monorepo), testsLabel(monorepo))
	}
}

func TestSourceLine(t *testing.T) {
	t.Parallel()

	lines := []string{"  first  ", "second"}
	for number, want := range map[int]string{0: "", 1: "first", 2: "second", 3: ""} {
		if got := sourceLine(lines, number); got != want {
			t.Errorf("sourceLine(%d) = %q, want %q", number, got, want)
		}
	}
}

func TestRefsFallBackWhenSourceIsUnreadable(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "gone.yaml")
	if got := uncoveredLineRefs(missing, []int{3}); len(got) != 1 || got[0] != missing+":3" {
		t.Errorf("line refs = %v", got)
	}
	if got := uncoveredBranchRefs(missing, []string{"3:if:true"}); len(got) != 1 || got[0] != missing+" (3:if:true)" {
		t.Errorf("branch refs = %v", got)
	}

	present := filepath.Join(t.TempDir(), "t.yaml")
	if err := os.WriteFile(present, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := uncoveredBranchRefs(present, []string{"legacy"}); len(got) != 1 || got[0] != present+" (legacy)" {
		t.Errorf("malformed branch ref = %v", got)
	}
}

type failAfterWriter struct{ writes int }

func (w *failAfterWriter) Write(p []byte) (int, error) {
	if w.writes == 0 {
		return 0, errors.New("stdout closed")
	}
	w.writes--
	return len(p), nil
}

func TestRunReturnsStdoutErrors(t *testing.T) {
	t.Parallel()

	for _, writes := range []int{0, 1} {
		chart, tests := halfCoveredChart(t)
		out := t.TempDir()
		err := Run([]string{
			"--chart", chart, "--tests", tests, "--verbose",
			"--go-coverprofile", filepath.Join(out, "c.out"), "--cobertura-file", filepath.Join(out, "c.xml"),
		}, &failAfterWriter{writes: writes})
		if err == nil || err.Error() != "stdout closed" {
			t.Errorf("after %d writes: err = %v, want stdout closed", writes, err)
		}
	}
}

func TestRunRendersEachTestWithItsOwnValues(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	chart := filepath.Join(base, "chart")
	writeTestFile(t, filepath.Join(chart, "Chart.yaml"), "apiVersion: v2\nname: demo\nversion: 0.1.0\n")
	writeTestFile(t, filepath.Join(chart, "values.yaml"), "on: false\n")
	writeTestFile(t, filepath.Join(chart, "templates", "a.yaml"), "{{- if .Values.on }}\na: on\n{{- end }}\n")
	writeTestFile(t, filepath.Join(chart, "templates", "b.yaml"), "{{- if .Values.on }}\nb: on\n{{- end }}\n")
	writeTestFile(t, filepath.Join(chart, "tests", "t_test.yaml"), `suite: t
templates:
  - templates/a.yaml
  - templates/b.yaml
tests:
  - it: turns a on
    template: templates/a.yaml
    set:
      on: true
`)

	var stdout bytes.Buffer
	err := Run([]string{
		"--chart", chart, "--tests", filepath.Join(chart, "tests"), "--max-scenarios", "1", "--verbose",
		"--go-coverprofile", filepath.Join(base, "c.out"), "--cobertura-file", filepath.Join(base, "c.xml"),
	}, &stdout)
	if err != nil {
		t.Fatal(err)
	}

	a := filepath.Join(chart, "templates", "a.yaml")
	b := filepath.Join(chart, "templates", "b.yaml")
	out := stdout.String()
	if strings.Contains(out, a+":1") {
		t.Errorf("a.yaml should be fully covered by the test's set value:\n%s", out)
	}
	if !strings.Contains(out, b+`:1 "{{- if .Values.on }}" (if:true)`) {
		t.Errorf("b.yaml's true branch should stay uncovered; the test only renders a.yaml:\n%s", out)
	}
}

func TestRunRendersWithHelmUnittestContext(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	chart := filepath.Join(base, "chart")
	writeTestFile(t, filepath.Join(chart, "Chart.yaml"), "apiVersion: v2\nname: demo\nversion: 0.1.0\n")
	writeTestFile(t, filepath.Join(chart, "templates", "a.yaml"), "{{- if .Capabilities.APIVersions.Has \"batch/v1\" }}\nbatch: on\n{{- end }}\n{{- if eq .Release.Name \"rel\" }}\nrel: on\n{{- end }}\n"+
		// helm-unittest's default is Kubernetes 1.20, so only the second test reaches 1.31.
		"{{- if semverCompare \">=1.25\" .Capabilities.KubeVersion.Version }}\nnew: on\n{{- end }}\n{{- if eq .Chart.Version \"9.9.9\" }}\nchart: on\n{{- end }}\n")
	writeTestFile(t, filepath.Join(chart, "tests", "t_test.yaml"), `suite: t
templates: ["*.yaml"]
tests:
  - it: uses helm-unittest's defaults
  - it: declares batch/v1 and a release name
    capabilities:
      apiVersions: [batch/v1]
      majorVersion: 1
      minorVersion: 31
    release:
      name: rel
    chart:
      version: 9.9.9
`)

	var stdout bytes.Buffer
	err := Run([]string{
		"--chart", chart, "--tests", filepath.Join(chart, "tests"), "--max-scenarios", "1", "--verbose",
		"--go-coverprofile", filepath.Join(base, "c.out"), "--cobertura-file", filepath.Join(base, "c.xml"),
	}, &stdout)
	if err != nil {
		t.Fatal(err)
	}
	// Both branches of each if are taken: the defaults miss them, the second test hits them.
	if strings.Contains(stdout.String(), "(if:") {
		t.Errorf("every branch should be covered:\n%s", stdout.String())
	}
}

func TestRunRendersUnpackedSubchartWithItsOwnContext(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	chart := filepath.Join(base, "chart")
	sub := filepath.Join(chart, "charts", "sub")
	writeTestFile(t, filepath.Join(chart, "Chart.yaml"), "apiVersion: v2\nname: demo\nversion: 0.1.0\n")
	writeTestFile(t, filepath.Join(chart, "values.yaml"), "sub:\n  enabled: true\nglobal:\n  region: parent\n")
	writeTestFile(t, filepath.Join(sub, "Chart.yaml"), "apiVersion: v2\nname: sub\nversion: 0.2.0\n")
	writeTestFile(t, filepath.Join(sub, "values.yaml"), "enabled: false\nown: from-sub\nglobal:\n  region: sub\n")
	writeTestFile(t, filepath.Join(sub, "data.txt"), "hello")
	writeTestFile(t, filepath.Join(sub, "templates", "_helpers.tpl"), `{{- define "sub.own" -}}{{ .Values.own }}{{- end -}}`)
	writeTestFile(t, filepath.Join(sub, "templates", "s.yaml"), `{{- if .Values.enabled }}
{{- if eq .Chart.Name "sub" }}chart{{ end }}
{{- if eq .Values.global.region "parent" }}global{{ end }}
{{- if eq (include "sub.own" .) "from-sub" }}include{{ end }}
{{- if eq (.Files.Get "data.txt") "hello" }}files{{ end }}
{{- end }}
`)
	writeTestFile(t, filepath.Join(chart, "tests", "t_test.yaml"), "suite: t\ntemplates: [charts/sub/templates/s.yaml]\ntests:\n  - it: renders\n")

	var stdout bytes.Buffer
	err := Run([]string{
		"--chart", chart, "--tests", filepath.Join(chart, "tests"), "--max-scenarios", "1", "--verbose",
		"--go-coverprofile", filepath.Join(base, "c.out"), "--cobertura-file", filepath.Join(base, "c.xml"),
	}, &stdout)
	if err != nil {
		t.Fatal(err)
	}
	// Helm renders a subchart with its own values, parent globals, Chart.yaml, helpers and files.
	got := stdout.String()
	if strings.Contains(got, "(if:true)") || strings.Count(got, "(if:false)") != 5 {
		t.Errorf("want every if:true covered and every if:false missed:\n%s", got)
	}
}

func TestRunAnswersLookupFromKubernetesProvider(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	chart := filepath.Join(base, "chart")
	writeTestFile(t, filepath.Join(chart, "Chart.yaml"), "apiVersion: v2\nname: demo\nversion: 0.1.0\n")
	writeTestFile(t, filepath.Join(chart, "templates", "s.yaml"), "{{- if lookup \"v1\" \"Secret\" .Release.Namespace \"existing\" }}\nreuse: true\n{{- else }}\ncreate: true\n{{- end }}\n")
	writeTestFile(t, filepath.Join(chart, "tests", "t_test.yaml"), `suite: t
templates: [templates/s.yaml]
release:
  namespace: default
tests:
  - it: has no cluster objects
  - it: finds the existing secret
    kubernetesProvider:
      scheme:
        "v1/Secret": {gvr: {version: v1, resource: secrets}, namespaced: true}
      objects:
        - {kind: Secret, apiVersion: v1, metadata: {name: existing, namespace: default}}
`)

	var stdout bytes.Buffer
	err := Run([]string{
		"--chart", chart, "--max-scenarios", "1", "--verbose",
		"--go-coverprofile", filepath.Join(base, "c.out"), "--cobertura-file", filepath.Join(base, "c.xml"),
	}, &stdout)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout.String(), "(if:") {
		t.Errorf("both lookup branches should be covered:\n%s", stdout.String())
	}
}

func TestRunBranchThresholdGate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		flags   []string
		wantErr []string
	}{
		{name: "met at the boundary", flags: []string{"--branch-threshold", "50"}},
		{name: "missed", flags: []string{"--branch-threshold", "50.5"}, wantErr: []string{"branch coverage 50.00 is below branch threshold 50.50"}},
		{
			name:    "both gates missed are both reported",
			flags:   []string{"--threshold", "60", "--branch-threshold", "70"},
			wantErr: []string{"coverage 50.00 is below threshold 60.00", "branch coverage 50.00 is below branch threshold 70.00"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := runHalfCovered(t, tc.flags...)
			if len(tc.wantErr) == 0 {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("want an error containing %q", tc.wantErr)
			}
			for _, want := range tc.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err, want)
				}
			}
		})
	}
}

func TestParseConfigBranchThresholdBounds(t *testing.T) {
	t.Parallel()

	for value, ok := range map[string]bool{"-1": false, "0": true, "100": true, "100.5": false} {
		chart, tests := halfCoveredChart(t)
		_, err := ParseConfig([]string{"--chart", chart, "--tests", tests, "--branch-threshold", value})
		if ok != (err == nil) {
			t.Errorf("--branch-threshold %s: err = %v", value, err)
		}
	}
}

func TestRunReportsCoverageReachedByTestsAlone(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	chart := filepath.Join(base, "chart")
	writeTestFile(t, filepath.Join(chart, "Chart.yaml"), "apiVersion: v2\nname: demo\nversion: 0.1.0\n")
	writeTestFile(t, filepath.Join(chart, "values.yaml"), "a: false\n")
	writeTestFile(t, filepath.Join(chart, "templates", "t.yaml"), "{{- if .Values.a }}\na: on\n{{- end }}\nkind: X\n")
	writeTestFile(t, filepath.Join(chart, "templates", "u.yaml"), "kind: U\n")
	// No test targets u.yaml, so only the suite's own render reaches it.
	writeTestFile(t, filepath.Join(chart, "tests", "t_test.yaml"), "suite: t\ntemplates: [t.yaml, u.yaml]\ntests:\n  - it: renders\n    template: t.yaml\n")
	markdown := filepath.Join(base, "c.md")

	var stdout bytes.Buffer
	// The one generated scenario flips a to true, which no test does.
	err := Run([]string{
		"--chart", chart, "--max-scenarios", "2", "--format", "markdown", "--markdown-file", markdown,
	}, &stdout)
	if err != nil {
		t.Fatal(err)
	}
	if want := "line-coverage=100.00% branch-coverage=100.00% line-coverage-tests=50.00% branch-coverage-tests=50.00%\n"; !strings.HasSuffix(stdout.String(), want) {
		t.Errorf("summary = %q, want suffix %q", stdout.String(), want)
	}
	md, err := os.ReadFile(markdown)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"| Line coverage | 100.00% | 50.00% |", "| Branch coverage | 100.00% | 50.00% |"} {
		if !strings.Contains(string(md), want) {
			t.Errorf("markdown missing %q:\n%s", want, md)
		}
	}
}

func TestRunComparesAgainstBaseline(t *testing.T) {
	t.Parallel()

	chart, tests := halfCoveredChart(t)
	out := t.TempDir()
	baseline := filepath.Join(out, "base.xml")
	writeTestFile(t, baseline, `<?xml version="1.0" encoding="UTF-8"?>
<coverage line-rate="0.6250" branch-rate="0.7500" version="helmcov"></coverage>
`)
	markdown := filepath.Join(out, "c.md")
	var stdout bytes.Buffer
	err := Run([]string{
		"--chart", chart, "--tests", tests, "--max-scenarios", "1",
		"--format", "markdown", "--markdown-file", markdown, "--baseline", baseline,
	}, &stdout)
	if err != nil {
		t.Fatal(err)
	}
	md, err := os.ReadFile(markdown)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"| Line coverage | 50.00% (-12.50) |", "| Branch coverage | 50.00% (-25.00) |"} {
		if !strings.Contains(string(md), want) {
			t.Errorf("markdown missing %q:\n%s", want, md)
		}
	}

	for name, content := range map[string]string{"missing": "", "not cobertura": "<coverage line-rate=\"x\"/>"} {
		path := filepath.Join(out, strings.ReplaceAll(name, " ", "-")+".xml")
		if content != "" {
			writeTestFile(t, path, content)
		}
		err := Run([]string{"--chart", chart, "--tests", tests, "--format", "markdown", "--markdown-file", markdown, "--baseline", path}, &stdout)
		if err == nil || !strings.Contains(err.Error(), "baseline") {
			t.Errorf("%s baseline: err = %v, want a baseline error", name, err)
		}
	}
}
