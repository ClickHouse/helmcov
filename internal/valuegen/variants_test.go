package valuegen

import (
	"reflect"
	"testing"
)

func TestGenerateReturnsBaseFirstAndOneChangePerScenario(t *testing.T) {
	t.Parallel()

	base := map[string]any{"a": true, "b": false, "list": []any{"x"}}
	cases := []struct {
		name  string
		limit int
		want  int
	}{
		{name: "limit below one still returns the base", limit: 0, want: 1},
		{name: "limit caps the variants", limit: 3, want: 3},
		{name: "every variant fits", limit: 10, want: 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			scenarios := Generate(base, Options{MaxScenarios: tc.limit, Seed: 7})
			if len(scenarios) != tc.want {
				t.Fatalf("len = %d, want %d", len(scenarios), tc.want)
			}
			if !reflect.DeepEqual(scenarios[0], base) {
				t.Errorf("first scenario = %v, want the base values %v", scenarios[0], base)
			}
			for i, scenario := range scenarios[1:] {
				if changed := changedKeys(base, scenario); len(changed) != 1 {
					t.Errorf("scenario %d changed %v, want exactly one key", i+1, changed)
				}
			}
		})
	}
}

func TestGenerateLeavesBaseUntouched(t *testing.T) {
	t.Parallel()

	base := map[string]any{"nested": map[string]any{"on": true}, "list": []any{"x"}}
	for _, scenario := range Generate(base, Options{MaxScenarios: 10}) {
		scenario["nested"].(map[string]any)["on"] = "mutated"
	}
	if base["nested"].(map[string]any)["on"] != true {
		t.Errorf("base was modified through a scenario: %v", base)
	}
}

func TestBuildVariantsCoversEveryKindInPathOrder(t *testing.T) {
	t.Parallel()

	base := map[string]any{
		"a.x":       true,
		"a-b":       false,
		"emptyList": []any{},
		"list":      []any{1},
		"strings":   []string{"s"},
		"noStrings": []string{},
		"emptyMap":  map[string]any{},
		"anyMap":    map[any]any{"k": true, 1: "dropped"},
		"emptyAny":  map[any]any{},
		"text":      "ignored",
	}
	want := []variant{
		{path: "a-b", value: true},
		{path: "a.x", value: false},
		{path: "anyMap.k", value: false},
		{path: "emptyAny", value: map[string]any{"generated": true}},
		{path: "emptyList", value: []any{"generated"}},
		{path: "emptyMap", value: map[string]any{"generated": true}},
		{path: "list", value: []any{}},
		{path: "noStrings", value: []any{"generated"}},
		{path: "strings", value: []any{}},
	}
	if got := buildVariants(base); !reflect.DeepEqual(got, want) {
		t.Errorf("buildVariants() =\n%#v\nwant\n%#v", got, want)
	}
}

func TestSetPathCreatesAndReplacesParents(t *testing.T) {
	t.Parallel()

	root := map[string]any{"scalar": "x", "keep": map[string]any{"other": 1}}
	setPath(root, "new.child", 1)
	setPath(root, "scalar.child", 2)
	setPath(root, "keep.added", 3)

	want := map[string]any{
		"new":    map[string]any{"child": 1},
		"scalar": map[string]any{"child": 2},
		"keep":   map[string]any{"other": 1, "added": 3},
	}
	if !reflect.DeepEqual(root, want) {
		t.Errorf("setPath result = %v, want %v", root, want)
	}
}

func TestDeepCopyConvertsStringSlices(t *testing.T) {
	t.Parallel()

	src := map[string]any{"s": []string{"a", "b"}, "l": []any{"c"}}
	got := deepCopy(src)
	if !reflect.DeepEqual(got, map[string]any{"s": []any{"a", "b"}, "l": []any{"c"}}) {
		t.Fatalf("deepCopy() = %#v", got)
	}
	got["l"].([]any)[0] = "changed"
	if src["l"].([]any)[0] != "c" {
		t.Errorf("copied slice shares storage with the source")
	}
}

func changedKeys(base, scenario map[string]any) []string {
	var changed []string
	for key, value := range scenario {
		if !reflect.DeepEqual(base[key], value) {
			changed = append(changed, key)
		}
	}
	return changed
}
