package chartloader

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadBundleLayersEachTestsValues(t *testing.T) {
	t.Parallel()

	chart := t.TempDir()
	writeFile(t, filepath.Join(chart, "Chart.yaml"), "apiVersion: v2\nname: demo\nversion: 0.1.0\n")
	writeFile(t, filepath.Join(chart, "values.yaml"), "mode: dev\nreplicas: 1\nnested:\n  a: 1\nlist:\n  - b: 1\n  - b: 2\ngone: here\n")
	writeFile(t, filepath.Join(chart, "tests", "values", "prod.yaml"), "mode: prod\nnested:\n  b: 2\n")
	writeFile(t, filepath.Join(chart, "tests", "s_test.yaml"), `suite: s
templates:
  - templates/a.yaml
  - templates/b.yaml
set:
  replicas: 2
  mode: suite-set
tests:
  - it: uses the suite values
  - it: layers a values file relative to the suite
    template: templates/b.yaml
    values:
      - values/prod.yaml
  - it: layers set paths in key order
    templates:
      - templates/a.yaml
    set:
      nested: {}
      nested.c: 3
      list[1].b: 5
      gone: null
  - it: is skipped and never loaded
    skip:
      reason: broken
    values:
      - missing.yaml
`)

	bundle, err := LoadBundle(chart, filepath.Join(chart, "tests"))
	if err != nil {
		t.Fatal(err)
	}
	suite := bundle.Suites[0]
	want := []struct {
		name      string
		templates []string
		values    map[string]any
	}{
		{
			name:      "uses the suite values",
			templates: []string{"templates/a.yaml", "templates/b.yaml"},
			values:    suiteValues(),
		},
		{
			// Suite set paths apply after every values file, so mode stays suite-set.
			name:      "layers a values file relative to the suite",
			templates: []string{"templates/b.yaml"},
			values: map[string]any{
				"mode": "suite-set", "replicas": 2, "nested": map[string]any{"a": 1, "b": 2},
				"list": []any{map[string]any{"b": 1}, map[string]any{"b": 2}}, "gone": "here",
			},
		},
		{
			// As in helm-unittest: maps merge, an index replaces the list, and null deletes.
			name:      "layers set paths in key order",
			templates: []string{"templates/a.yaml"},
			values: map[string]any{
				"mode": "suite-set", "replicas": 2, "nested": map[string]any{"a": 1, "c": 3},
				"list": []any{nil, map[string]any{"b": 5}},
			},
		},
	}
	if len(suite.Tests) != len(want) {
		t.Fatalf("got %d tests, want %d", len(suite.Tests), len(want))
	}
	for i, w := range want {
		test := suite.Tests[i]
		if test.Name != w.name {
			t.Errorf("test %d name = %q, want %q", i, test.Name, w.name)
		}
		if got := test.TemplatesIn(suite); !reflect.DeepEqual(got, w.templates) {
			t.Errorf("%s: templates = %v, want %v", w.name, got, w.templates)
		}
		if !reflect.DeepEqual(test.MergedValues, w.values) {
			t.Errorf("%s: values = %#v, want %#v", w.name, test.MergedValues, w.values)
		}
	}
	if !reflect.DeepEqual(suite.MergedValues, suiteValues()) {
		t.Errorf("suite values changed by its tests: %#v", suite.MergedValues)
	}
}

func TestLoadBundleReportsMissingTestValuesFile(t *testing.T) {
	t.Parallel()

	chart := t.TempDir()
	writeFile(t, filepath.Join(chart, "Chart.yaml"), "apiVersion: v2\nname: demo\nversion: 0.1.0\n")
	writeFile(t, filepath.Join(chart, "tests", "s_test.yaml"), "suite: s\ntests:\n  - it: x\n    values:\n      - nope.yaml\n")
	if _, err := LoadBundle(chart, filepath.Join(chart, "tests")); err == nil {
		t.Fatal("want an error for a missing test values file")
	}
}

func suiteValues() map[string]any {
	return map[string]any{
		"mode": "suite-set", "replicas": 2, "nested": map[string]any{"a": 1},
		"list": []any{map[string]any{"b": 1}, map[string]any{"b": 2}}, "gone": "here",
	}
}

func TestLoadBundleSkipsSkippedSuites(t *testing.T) {
	t.Parallel()

	chart := t.TempDir()
	writeFile(t, filepath.Join(chart, "Chart.yaml"), "apiVersion: v2\nname: demo\nversion: 0.1.0\n")
	writeFile(t, filepath.Join(chart, "tests", "a_test.yaml"), "suite: skipped\nskip:\n  reason: wip\nvalues:\n  - missing.yaml\n")
	writeFile(t, filepath.Join(chart, "tests", "b_test.yaml"), "suite: kept\n")
	bundle, err := LoadBundle(chart, filepath.Join(chart, "tests"))
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Suites) != 1 || bundle.Suites[0].Name != "kept" {
		t.Errorf("suites = %v, want only the unskipped one", bundle.Suites)
	}
}
