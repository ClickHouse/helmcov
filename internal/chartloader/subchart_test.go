package chartloader

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSubchartScopeValues(t *testing.T) {
	t.Parallel()

	sub := Subchart{Meta: ChartMeta{Name: "sub"}}
	cases := []struct {
		name   string
		parent map[string]any
		want   map[string]any
	}{
		{
			name:   "nothing under the subchart name",
			parent: map[string]any{"other": map[string]any{"replicas": 9.0}},
			want:   map[string]any{},
		},
		{
			name:   "the parent's values under the subchart name",
			parent: map[string]any{"sub": map[string]any{"replicas": 3.0, "global": map[string]any{"zone": "a"}}},
			want:   map[string]any{"replicas": 3.0, "global": map[string]any{"zone": "a"}},
		},
		{
			name:   "parent globals override the subchart's",
			parent: map[string]any{"sub": map[string]any{"global": map[string]any{"region": "sub", "zone": "a"}}, "global": map[string]any{"region": "parent"}},
			want:   map[string]any{"global": map[string]any{"region": "parent", "zone": "a"}},
		},
		{
			name:   "parent globals without subchart globals",
			parent: map[string]any{"global": map[string]any{"region": "parent"}},
			want:   map[string]any{"global": map[string]any{"region": "parent"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := sub.ScopeValues(tc.parent); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ScopeValues() = %#v\nwant %#v", got, tc.want)
			}
		})
	}
	parent := map[string]any{"sub": map[string]any{"global": map[string]any{"zone": "a"}}, "global": map[string]any{"region": "parent"}}
	sub.ScopeValues(parent)
	if !reflect.DeepEqual(parent["sub"], map[string]any{"global": map[string]any{"zone": "a"}}) {
		t.Errorf("ScopeValues changed the parent's values: %#v", parent)
	}
}

func TestLoadBundleCoalescesSubchartDefaults(t *testing.T) {
	t.Parallel()

	chart := t.TempDir()
	write := func(path, content string) {
		t.Helper()
		full := filepath.Join(chart, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("Chart.yaml", "name: demo\n")
	write("values.yaml", "sub:\n  image:\n    tag: parent\nflag: false\n")
	write("charts/sub/Chart.yaml", "name: sub\n")
	write("charts/sub/values.yaml", "image:\n  repo: default\n  tag: v1\nreplicas: 1\n")
	write("charts/plain/Chart.yaml", "name: plain\n")
	write("charts/packaged-1.0.0.tgz", "not read")
	write("tests/t_test.yaml", `suite: t
templates: [charts/sub/templates/*.yaml]
tests:
  - it: unsets the repo
    set:
      sub.image.repo: null
`)
	write("charts/sub/templates/a.yaml", "")
	write("charts/sub/templates/_helpers.tpl", "")

	bundle, err := LoadBundle(chart, filepath.Join(chart, "tests"))
	if err != nil {
		t.Fatal(err)
	}
	wantSub := map[string]any{"image": map[string]any{"repo": "default", "tag": "parent"}, "replicas": 1.0}
	if want := map[string]any{"sub": wantSub, "plain": map[string]any{}, "flag": false}; !reflect.DeepEqual(bundle.Values, want) {
		t.Errorf("Values = %#v\nwant %#v", bundle.Values, want)
	}
	suite := bundle.Suites[0]
	if !reflect.DeepEqual(suite.Templates, []string{"charts/sub/templates/a.yaml"}) {
		t.Errorf("Templates = %v", suite.Templates)
	}
	// As in Helm, a test's null removes the subchart's default even when the parent's values.yaml holds the map.
	if got, want := suite.Tests[0].MergedValues["sub"], map[string]any{"image": map[string]any{"tag": "parent"}, "replicas": 1.0}; !reflect.DeepEqual(got, want) {
		t.Errorf("test values for sub = %#v, want %#v", got, want)
	}
}

func TestCoalesceSubchartDefaultsKeepsNonMapValues(t *testing.T) {
	t.Parallel()

	chart := t.TempDir()
	if err := os.MkdirAll(filepath.Join(chart, "charts", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chart, "charts", "sub", "Chart.yaml"), []byte("name: sub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	values := map[string]any{"sub": "disabled"}
	if err := coalesceSubchartDefaults(chart, values); err != nil || values["sub"] != "disabled" {
		t.Errorf("coalesce = %#v, %v; want the parent's string kept", values, err)
	}
	if err := coalesceSubchartDefaults(t.TempDir(), values); err != nil {
		t.Errorf("a chart without charts/ = %v", err)
	}
}

func TestLoadSubchartFor(t *testing.T) {
	t.Parallel()

	chart := t.TempDir()
	write := func(path, content string) {
		full := filepath.Join(chart, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("charts/full/Chart.yaml", "name: full\nversion: 1.0.0\n")
	write("charts/full/values.yaml", "a: 1\n")
	write("charts/novalues/Chart.yaml", "name: novalues\n")
	write("charts/nochart/templates/x.yaml", "")
	write("charts/badchart/Chart.yaml", "name: [\n")
	write("charts/badvalues/Chart.yaml", "name: badvalues\n")
	write("charts/badvalues/values.yaml", "a: [\n")

	cases := []struct {
		template string
		want     Subchart
		ok       bool
		err      string
	}{
		{template: "charts/full/templates/x.yaml", ok: true, want: Subchart{Path: filepath.Join(chart, "charts", "full"), Meta: ChartMeta{Name: "full", Version: "1.0.0"}, Values: map[string]any{"a": 1.0}}},
		{template: "charts/novalues/templates/x.yaml", ok: true, want: Subchart{Path: filepath.Join(chart, "charts", "novalues"), Meta: ChartMeta{Name: "novalues"}, Values: map[string]any{}}},
		{template: "charts/nochart/templates/x.yaml"},
		{template: "charts/missing/templates/x.yaml"},
		{template: "templates/x.yaml"},
		{template: "charts/full/x.yaml"},
		{template: "charts//templates/x.yaml"},
		{template: "charts/badchart/templates/x.yaml", err: "parse subchart badchart Chart.yaml"},
		{template: "charts/full/templates", ok: false},
		{template: "charts/badvalues/templates/x.yaml", err: "load subchart badvalues values"},
	}
	for _, tc := range cases {
		t.Run(tc.template, func(t *testing.T) {
			t.Parallel()
			got, ok, err := LoadSubchartFor(chart, tc.template)
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("err = %v, want %q", err, tc.err)
				}
				return
			}
			if err != nil || ok != tc.ok || !reflect.DeepEqual(got, tc.want) {
				t.Errorf("LoadSubchartFor() = %#v, %v, %v; want %#v, %v", got, ok, err, tc.want, tc.ok)
			}
		})
	}
}
