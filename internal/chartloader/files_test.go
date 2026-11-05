package chartloader

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func TestLoadChartFilesMatchesHelm(t *testing.T) {
	t.Parallel()

	chart := t.TempDir()
	for path, content := range map[string]string{
		"Chart.yaml": "x", "values.yaml": "x", "values.schema.json": "x", "Chart.lock": "x",
		"README.md": "readme", "NOTES.md": "root", "docs/NOTES.md": "nested", "bom.txt": "\xef\xbb\xbfbom",
		"files/a.txt": "a", "files/sub/b.txt": "b", "tests/t_test.yaml": "t",
		"templates/x.yaml": "x", "charts/sub/Chart.yaml": "x",
		"secret.key": "x", "build/out.txt": "x", "docs/build": "file, not dir",
		".helmignore": "# comment\n*.key\nbuild/\n/files/sub/*\n/NOTES.md\n",
	} {
		writeFile(t, filepath.Join(chart, path), content)
	}
	if err := os.Symlink(filepath.Join(chart, "files"), filepath.Join(chart, "linked")); err != nil {
		t.Fatal(err)
	}

	files, err := LoadChartFiles(chart)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		".helmignore", "README.md", "bom.txt", "docs/NOTES.md", "docs/build", "files/a.txt",
		"linked/a.txt", "linked/sub/b.txt", "tests/t_test.yaml",
	}
	if got := sortedNames(files); !reflect.DeepEqual(got, want) {
		t.Errorf("files = %v, want %v", got, want)
	}
	if files.Get("files/a.txt") != "a" || files.Get("a.txt") != "" || len(files.GetBytes("missing")) != 0 {
		t.Errorf("Get is keyed from the chart root and returns nothing for missing files")
	}
	if files.Get("bom.txt") != "bom" {
		t.Errorf("bom.txt = %q, want the BOM stripped", files.Get("bom.txt"))
	}
}

func TestLoadChartFilesRejectsWhatHelmRejects(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		// Helm ignores every path a negated rule does not match, Chart.yaml included.
		"negation":    "*.key\n!keep.key\n",
		"double star": "files/**\n",
		"bad glob":    "files/[\n",
	}
	for name, rules := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			chart := t.TempDir()
			writeFile(t, filepath.Join(chart, "Chart.yaml"), "x")
			writeFile(t, filepath.Join(chart, ".helmignore"), rules)
			if _, err := LoadChartFiles(chart); err == nil {
				t.Fatalf("want an error for .helmignore %q", rules)
			}
		})
	}
}

func sortedNames(files ChartFiles) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func TestChartFilesGlobMatchesHelm(t *testing.T) {
	t.Parallel()

	files := ChartFiles{
		"files/a.txt": nil, "files/b.txt": nil, "files/sub/c.yaml": nil, "files/sub/deep/d.yaml": nil, "README.md": nil,
		"files/café.txt": nil, "files/^.txt": nil,
	}
	cases := map[string][]string{
		"files/*":            {"files/^.txt", "files/a.txt", "files/b.txt", "files/café.txt"},
		"files/**":           {"files/^.txt", "files/a.txt", "files/b.txt", "files/café.txt", "files/sub/c.yaml", "files/sub/deep/d.yaml"},
		"**.yaml":            {"files/sub/c.yaml", "files/sub/deep/d.yaml"},
		"files/café.txt":     {"files/café.txt"},
		"files/[^a].txt":     {"files/^.txt", "files/a.txt"},
		"files/{a,b}.txt":    {"files/a.txt", "files/b.txt"},
		"files/?.txt":        {"files/^.txt", "files/a.txt", "files/b.txt"},
		"files/[!b].txt":     {"files/^.txt", "files/a.txt"},
		"files/[ab].txt":     {"files/a.txt", "files/b.txt"},
		`README\.md`:         {"README.md"},
		"README.md,x":        {},
		"files/[":            {"README.md", "files/^.txt", "files/a.txt", "files/b.txt", "files/café.txt", "files/sub/c.yaml", "files/sub/deep/d.yaml"},
		`README.md\`:         {"README.md", "files/^.txt", "files/a.txt", "files/b.txt", "files/café.txt", "files/sub/c.yaml", "files/sub/deep/d.yaml"},
		"files/{a,b.txt":     {"README.md", "files/^.txt", "files/a.txt", "files/b.txt", "files/café.txt", "files/sub/c.yaml", "files/sub/deep/d.yaml"},
		"files/a.txt}":       {"README.md", "files/^.txt", "files/a.txt", "files/b.txt", "files/café.txt", "files/sub/c.yaml", "files/sub/deep/d.yaml"},
		"files/*/{c,d}.yaml": {"files/sub/c.yaml"},
	}
	for pattern, want := range cases {
		got := slices.Sorted(func(yield func(string) bool) {
			for name := range files.Glob(pattern) {
				if !yield(name) {
					return
				}
			}
		})
		if len(got) == 0 {
			got = []string{}
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Glob(%q) = %v, want %v", pattern, got, want)
		}
	}
}

func TestChartFilesLinesAsConfigAndAsSecrets(t *testing.T) {
	t.Parallel()

	files := ChartFiles{"files/b.txt": []byte("x\ny\n"), "conf/a.txt": []byte("one"), "empty": []byte("")}
	if got := files.Lines("files/b.txt"); !reflect.DeepEqual(got, []string{"x", "y"}) {
		t.Errorf("Lines = %#v", got)
	}
	if got := files.Lines("empty"); len(got) != 0 || got == nil {
		t.Errorf("Lines(empty) = %#v, want an empty slice", got)
	}
	if got := files.Lines("missing"); len(got) != 0 {
		t.Errorf("Lines(missing) = %#v", got)
	}
	if got, want := files.AsConfig(), "a.txt: one\nb.txt: |\n  x\n  y\nempty: \"\""; got != want {
		t.Errorf("AsConfig() =\n%s\nwant\n%s", got, want)
	}
	if got, want := files.AsSecrets(), "a.txt: b25l\nb.txt: eAp5Cg==\nempty: \"\""; got != want {
		t.Errorf("AsSecrets() =\n%s\nwant\n%s", got, want)
	}
	if (ChartFiles{}).AsConfig() != "" || (ChartFiles{}).AsSecrets() != "" {
		t.Error("no files should render as an empty string")
	}
}

func TestNegatedHelmignoreRuleIgnoresEverythingElse(t *testing.T) {
	t.Parallel()

	chart := t.TempDir()
	writeFile(t, filepath.Join(chart, "Chart.yaml"), "x")
	writeFile(t, filepath.Join(chart, "README.md"), "x")
	writeFile(t, filepath.Join(chart, "docs", "a.txt"), "x")
	writeFile(t, filepath.Join(chart, ".helmignore"), "!Chart.yaml\n")
	files, err := LoadChartFiles(chart)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Errorf("files = %v, want none: Helm ignores every path a negated rule does not match", sortedNames(files))
	}
}
