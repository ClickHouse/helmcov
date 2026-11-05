// Package e2e builds the helmcov binary and runs it against the charts in testdata.
package e2e

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ClickHouse/helmcov/internal/chartloader"
	"github.com/ClickHouse/helmcov/internal/instrumentation"
)

var update = flag.Bool("update", false, "rewrite golden files with the current output")

var binary string

func TestMain(m *testing.M) {
	flag.Parse()
	dir, err := os.MkdirTemp("", "helmcov-e2e")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	binary = filepath.Join(dir, "helmcov")
	build := exec.Command("go", "build", "-o", binary, "../cmd/helmcov")
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "build helmcov:", err)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// One scenario keeps the run to the suites' own values, so the expected coverage
// can be checked by hand against the templates.
func TestConstructsChartCoverage(t *testing.T) {
	t.Parallel()

	out := t.TempDir()
	profile := filepath.Join(out, "coverage.out")
	cmd := exec.Command(binary,
		"--chart", "testdata/constructs",
		"--tests", "testdata/constructs/tests",
		"--max-scenarios", "1",
		"--go-coverprofile", profile,
		"--cobertura-file", filepath.Join(out, "coverage.xml"),
		"--verbose",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("helmcov: %v\n%s", err, stderr.String())
	}

	assertGolden(t, "testdata/constructs.verbose.golden", stdout.Bytes())
	got, err := os.ReadFile(profile)
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "testdata/constructs.coverprofile.golden", got)
}

// helmcov renders with its own copy of Helm's function map and context, so
// compare it with real Helm to catch drift.
func TestRenderMatchesHelmTemplate(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("helm"); err != nil {
		if os.Getenv("CI") == "true" {
			t.Fatal("helm must be installed in CI so the parity check runs")
		}
		t.Skip("helm not on PATH")
	}

	for _, chart := range []string{"testdata/constructs", "testdata/parity"} {
		t.Run(filepath.Base(chart), func(t *testing.T) {
			t.Parallel()
			assertRendersLikeHelm(t, chart)
		})
	}
}

func assertRendersLikeHelm(t *testing.T, chart string) {
	t.Helper()
	bundle, err := chartloader.LoadBundle(chart, filepath.Join(chart, "tests"))
	if err != nil {
		t.Fatal(err)
	}
	helmDocs := helmTemplate(t, chart, bundle.Chart.Name)
	shared, err := chartloader.LoadTemplateFiles(chart)
	if err != nil {
		t.Fatal(err)
	}
	files, err := chartloader.LoadChartFiles(chart)
	if err != nil {
		t.Fatal(err)
	}

	templates, err := filepath.Glob(filepath.Join(chart, "templates", "[^_]*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(helmDocs) != len(templates) {
		t.Fatalf("helm rendered %d documents for %d templates; every template must render exactly one document so it is compared", len(helmDocs), len(templates))
	}

	for name, want := range helmDocs {
		path := filepath.Join(chart, "templates", name)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		_, got, err := instrumentation.NewExecutor().RenderAndTrace(path, string(content),
			chartloader.HelmRenderValues(chartloader.RenderOptions{
				Chart:        bundle.Chart,
				Values:       bundle.Values,
				ChartPath:    chart,
				TemplatePath: path,
				KubeVersion:  "1.28.0",
				Files:        files,
			}), shared)
		if err != nil {
			t.Fatalf("render %s: %v", name, err)
		}
		if strings.TrimSpace(got) != want {
			t.Errorf("%s differs from helm template\n--- helm\n%s\n--- helmcov\n%s", name, want, strings.TrimSpace(got))
		}
	}
}

// helmTemplate returns each rendered document keyed by its template file name.
func helmTemplate(t *testing.T, chart, name string) map[string]string {
	t.Helper()
	out, err := exec.Command("helm", "template", "release-name", chart, "--kube-version", "1.28.0").Output()
	if err != nil {
		t.Fatalf("helm template: %v", err)
	}
	docs := map[string]string{}
	for _, doc := range strings.Split(string(out), "\n---\n") {
		doc = strings.TrimPrefix(strings.TrimSpace(doc), "---\n")
		header, body, ok := strings.Cut(doc, "\n")
		if !ok || !strings.HasPrefix(header, "# Source: ") {
			continue
		}
		file := strings.TrimPrefix(header, "# Source: "+name+"/templates/")
		if _, dup := docs[file]; dup {
			t.Fatalf("%s rendered more than one document; Helm reorders those, so split it", file)
		}
		docs[file] = strings.TrimSpace(body)
	}
	if len(docs) == 0 {
		t.Fatalf("no documents in helm output:\n%s", out)
	}
	return docs
}

func assertGolden(t *testing.T, path string, got []byte) {
	t.Helper()
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run go test ./e2e -update to create it): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s is out of date; run go test ./e2e -update and review the diff\n--- want\n%s\n--- got\n%s", path, want, got)
	}
}
