package chartloader

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestIsPackagedSubchartTemplate(t *testing.T) {
	t.Parallel()

	chart := t.TempDir()
	if err := os.MkdirAll(filepath.Join(chart, "charts", "unpacked"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(chart, "charts", "plainfile"), "")

	cases := map[string]bool{
		"charts/packaged/templates/x.yaml":     true,
		"charts/plainfile/templates/x.yaml":    true,
		"charts/unpacked/templates/x.yaml":     false,
		"templates/x.yaml":                     false,
		"charts//templates/x.yaml":             false,
		"charts/packaged/other/x.yaml":         false,
		"vendor/packaged/templates/x.yaml":     false,
		"charts/packaged/templates":            false,
		"charts/packaged/templates/sub/x.yaml": true,
	}
	for path, want := range cases {
		if got := IsPackagedSubchartTemplate(chart, path); got != want {
			t.Errorf("IsPackagedSubchartTemplate(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestMatchChartFileGlob(t *testing.T) {
	t.Parallel()

	cases := []struct {
		pattern, name string
		want          bool
	}{
		{"files/*.txt", "files/a.txt", true},
		{"./files/*", "files/a.txt", true},
		{"files/*", "files", true},
		{"files/*", "files/sub/a.txt", false},
		{"files/*", "other/a.txt", false},
		{"files/**", "files", true},
		{"files/**", "files/sub/a.txt", true},
		{"files/**", "filesystem/a.txt", false},
		{`files\*.txt`, "files/a.txt", true},
		{"*.txt", "files/a.txt", false},
	}
	for _, tc := range cases {
		got, err := matchChartFileGlob(tc.pattern, tc.name)
		if err != nil {
			t.Fatalf("matchChartFileGlob(%q, %q): %v", tc.pattern, tc.name, err)
		}
		if got != tc.want {
			t.Errorf("matchChartFileGlob(%q, %q) = %v, want %v", tc.pattern, tc.name, got, tc.want)
		}
	}
	if _, err := matchChartFileGlob("files/[", "files/a"); err == nil {
		t.Error("want an error for a malformed pattern")
	}
}

func TestChartFilesLinesAndAsSecrets(t *testing.T) {
	t.Parallel()

	files := ChartFiles{}.WithFile("b.txt", []byte("x\ny")).WithFile("a.txt", []byte(""))

	if got, err := files.Lines("b.txt"); err != nil || !reflect.DeepEqual(got, []string{"x", "y"}) {
		t.Errorf("Lines(b.txt) = %v, %v", got, err)
	}
	if got, err := files.Lines("a.txt"); err != nil || len(got) != 0 {
		t.Errorf("Lines(a.txt) = %#v, %v; want empty", got, err)
	}
	if _, err := files.Lines("missing.txt"); err == nil {
		t.Error("Lines(missing) should fail")
	}
	got, err := files.AsSecrets()
	if err != nil {
		t.Fatal(err)
	}
	if want := "a.txt: \"\"\nb.txt: \"x\\ny\""; got != want {
		t.Errorf("AsSecrets() = %q, want %q", got, want)
	}
	if empty, _ := (ChartFiles{}).AsSecrets(); empty != "" {
		t.Errorf("AsSecrets() on no files = %q, want empty", empty)
	}
}

func TestLoadBundleAppliesSetPaths(t *testing.T) {
	t.Parallel()

	chart := t.TempDir()
	writeFile(t, filepath.Join(chart, "Chart.yaml"), "apiVersion: v2\nname: demo\nversion: 0.1.0\n")
	// The integer key makes YAML decode "odd" with non-string keys.
	writeFile(t, filepath.Join(chart, "values.yaml"), "scalar: x\nodd:\n  1: one\n  name: kept\n")
	writeFile(t, filepath.Join(chart, "tests", "extra.yaml"), "fromFile:\n  2: two\n")
	writeFile(t, filepath.Join(chart, "tests", "s_test.yaml"), `suite: s
values:
  - extra.yaml
set:
  scalar.child: 1
  new.deep.key: 2
  odd.added: 3
`)

	bundle, err := LoadBundle(chart, filepath.Join(chart, "tests"))
	if err != nil {
		t.Fatal(err)
	}
	values := bundle.Suites[0].MergedValues
	if got := values["scalar"]; !reflect.DeepEqual(got, map[string]any{"child": 1}) {
		t.Errorf("scalar = %v, want it replaced by a map", got)
	}
	if got := values["new"]; !reflect.DeepEqual(got, map[string]any{"deep": map[string]any{"key": 2}}) {
		t.Errorf("new = %v", got)
	}
	want := map[string]any{"1": "one", "name": "kept", "added": 3}
	if got := values["odd"]; !reflect.DeepEqual(got, want) {
		t.Errorf("odd = %#v, want %#v", got, want)
	}
	if got := values["fromFile"]; !reflect.DeepEqual(got, map[string]any{"2": "two"}) {
		t.Errorf("fromFile = %#v, want string keys", got)
	}
}

func TestLoadBundleReportsBadValues(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		values, suite, wantErr string
	}{
		"invalid values.yaml":  {values: "a: [", suite: "suite: s\n", wantErr: "yaml"},
		"missing suite values": {values: "", suite: "suite: s\nvalues:\n  - nope.yaml\n", wantErr: "load suite values"},
		"invalid suite":        {values: "", suite: "suite: [", wantErr: "parse suite"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			chart := t.TempDir()
			writeFile(t, filepath.Join(chart, "Chart.yaml"), "apiVersion: v2\nname: demo\nversion: 0.1.0\n")
			writeFile(t, filepath.Join(chart, "values.yaml"), tc.values)
			writeFile(t, filepath.Join(chart, "tests", "s_test.yaml"), tc.suite)
			_, err := LoadBundle(chart, filepath.Join(chart, "tests"))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestLoadYAMLMapTreatsEmptyDocumentsAsEmptyMaps(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	for name, content := range map[string]string{"empty.yaml": "", "comment.yaml": "# nothing\n"} {
		path := filepath.Join(dir, name)
		writeFile(t, path, content)
		got, err := loadYAMLMap(path)
		if err != nil || got == nil || len(got) != 0 {
			t.Errorf("%s: loadYAMLMap = %#v, %v; want an empty map", name, got, err)
		}
	}
}

func TestTemplateContextWithoutChartPath(t *testing.T) {
	t.Parallel()

	got := templateContext("", "/tmp/x/deploy.yaml")
	want := map[string]any{"Name": "deploy.yaml", "BasePath": ""}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("templateContext() = %v, want %v", got, want)
	}
	nested := templateContext("/c", "/c/templates/sub/deploy.yaml")
	if nested["Name"] != "sub/deploy.yaml" || nested["BasePath"] != "sub" {
		t.Errorf("nested templateContext() = %v", nested)
	}
}

func TestKubeMajorMinor(t *testing.T) {
	t.Parallel()

	for version, want := range map[string][2]string{"v1.29.3": {"1", "29"}, "v1": {"1", "0"}, "2.5": {"2", "5"}} {
		major, minor := kubeMajorMinor(version)
		if major != want[0] || minor != want[1] {
			t.Errorf("kubeMajorMinor(%q) = %s.%s, want %s.%s", version, major, minor, want[0], want[1])
		}
	}
}

func TestResolveTemplatePathRejectsMissingAbsolutePath(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "gone.yaml")
	if _, err := ResolveTemplatePath("/chart", missing); err == nil || !strings.Contains(err.Error(), "template not found") {
		t.Fatalf("err = %v, want template not found", err)
	}
}

func TestMergeIntoReplacesNonMapsAndMergesMaps(t *testing.T) {
	t.Parallel()

	dst := map[string]any{"scalar": "x", "nested": map[string]any{"keep": 1, "over": 1}}
	mergeInto(dst, map[string]any{
		"scalar": map[string]any{"now": "map"},
		"nested": map[string]any{"over": 2},
		"added":  true,
	})
	want := map[string]any{
		"scalar": map[string]any{"now": "map"},
		"nested": map[string]any{"keep": 1, "over": 2},
		"added":  true,
	}
	if !reflect.DeepEqual(dst, want) {
		t.Errorf("mergeInto() = %v, want %v", dst, want)
	}
}
