package chartloader

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTestContextLayersSuiteAndTestOverrides(t *testing.T) {
	t.Parallel()

	chart := t.TempDir()
	writeFile(t, filepath.Join(chart, "Chart.yaml"), "apiVersion: v2\nname: demo\nversion: 0.1.0\n")
	writeFile(t, filepath.Join(chart, "tests", "s_test.yaml"), `suite: s
release:
  name: suite-release
  namespace: suite-ns
  revision: 2
  upgrade: true
capabilities:
  apiVersions: [batch/v1]
chart:
  version: 1.0.0
tests:
  - it: inherits the suite
  - it: overrides
    release:
      namespace: test-ns
      revision: 3
      upgrade: false
    capabilities:
      majorVersion: 1
      minorVersion: 31
      apiVersions: [custom/v1]
    chart:
      appVersion: "9.9"
  - it: clears the api versions
    capabilities:
      apiVersions: null
  - it: adds nothing with an empty list
    capabilities:
      apiVersions: []
`)
	writeFile(t, filepath.Join(chart, "tests", "t_test.yaml"), "suite: plain\ntests:\n  - it: uses helm-unittest defaults\n")

	bundle, err := LoadBundle(chart, filepath.Join(chart, "tests"))
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Suite{}
	for _, s := range bundle.Suites {
		byName[s.Name] = s
	}

	cases := []struct {
		name string
		got  UnittestContext
		want UnittestContext
	}{
		{
			name: "helm-unittest defaults",
			got:  byName["plain"].Tests[0].ContextIn(byName["plain"]),
			want: UnittestContext{Release: release("RELEASE-NAME", "NAMESPACE", 0, false), APIVersions: APIVersionSet{}},
		},
		{
			name: "suite values",
			got:  byName["s"].Context(),
			want: UnittestContext{Release: release("suite-release", "suite-ns", 2, true), APIVersions: APIVersionSet{"batch/v1"}, ChartVersion: "1.0.0"},
		},
		{
			name: "test inherits the suite",
			got:  byName["s"].Tests[0].ContextIn(byName["s"]),
			want: UnittestContext{Release: release("suite-release", "suite-ns", 2, true), APIVersions: APIVersionSet{"batch/v1"}, ChartVersion: "1.0.0"},
		},
		{
			// As in helm-unittest: zero values (upgrade: false) don't override, and test API versions come first, then the suite's.
			name: "test overrides field by field",
			got:  byName["s"].Tests[1].ContextIn(byName["s"]),
			want: UnittestContext{
				Release: release("suite-release", "test-ns", 3, true), KubeVersion: "v1.31.0",
				APIVersions: APIVersionSet{"custom/v1", "batch/v1"}, ChartVersion: "1.0.0", ChartAppVersion: "9.9",
			},
		},
		{
			name: "null clears the API versions",
			got:  byName["s"].Tests[2].ContextIn(byName["s"]),
			want: UnittestContext{Release: release("suite-release", "suite-ns", 2, true), APIVersions: APIVersionSet{}, ChartVersion: "1.0.0"},
		},
		{
			name: "an empty list keeps the suite's",
			got:  byName["s"].Tests[3].ContextIn(byName["s"]),
			want: UnittestContext{Release: release("suite-release", "suite-ns", 2, true), APIVersions: APIVersionSet{"batch/v1"}, ChartVersion: "1.0.0"},
		},
	}
	for _, tc := range cases {
		if !reflect.DeepEqual(tc.got, tc.want) {
			t.Errorf("%s: context = %#v\nwant %#v", tc.name, tc.got, tc.want)
		}
	}
}

func release(name, namespace string, revision int, upgrade bool) map[string]any {
	return map[string]any{
		"Name": name, "Namespace": namespace, "Revision": revision,
		"IsInstall": !upgrade, "IsUpgrade": upgrade, "Service": "Helm",
	}
}

func TestLoadBundleExpandsTemplateGlobsLikeHelmUnittest(t *testing.T) {
	t.Parallel()

	chart := t.TempDir()
	writeFile(t, filepath.Join(chart, "Chart.yaml"), "apiVersion: v2\nname: demo\nversion: 0.1.0\n")
	for _, f := range []string{"a.yaml", "b.yaml", "sub/s.yaml", "_helpers.tpl", "NOTES.txt"} {
		writeFile(t, filepath.Join(chart, "templates", f), "x")
	}
	writeFile(t, filepath.Join(chart, "tests", "s_test.yaml"), `suite: s
templates:
  - "*.yaml"
  - templates/a.yaml
  - charts/packaged/templates/*.yaml
tests:
  - it: narrows to a directory
    template: templates/sub/*
  - it: uses a list of globs
    templates: ["a.yaml", "templates/b*", "**s.yaml"]
`)
	bundle, err := LoadBundle(chart, filepath.Join(chart, "tests"))
	if err != nil {
		t.Fatal(err)
	}
	suite := bundle.Suites[0]
	if want := []string{"templates/a.yaml", "templates/b.yaml", "templates/sub/s.yaml"}; !reflect.DeepEqual(suite.Templates, want) {
		t.Errorf("suite templates = %v, want %v", suite.Templates, want)
	}
	if got, want := suite.Tests[0].TemplatesIn(suite), []string{"templates/sub/s.yaml"}; !reflect.DeepEqual(got, want) {
		t.Errorf("test 0 templates = %v, want %v", got, want)
	}
	// ** needs at least one character and gets no templates/ prefix, as in helm-unittest.
	if got, want := suite.Tests[1].TemplatesIn(suite), []string{"a.yaml", "templates/b.yaml", "templates/sub/s.yaml"}; !reflect.DeepEqual(got, want) {
		t.Errorf("test 1 templates = %v, want %v", got, want)
	}

	writeFile(t, filepath.Join(chart, "tests", "s_test.yaml"), "suite: s\ntemplates: [\"nothing-*.yaml\"]\n")
	if _, err := LoadBundle(chart, filepath.Join(chart, "tests")); err == nil || !strings.Contains(err.Error(), "matches no templates") {
		t.Errorf("err = %v, want a no-match error", err)
	}
}

func TestUnittestContextFillsAMissingVersionPart(t *testing.T) {
	t.Parallel()

	cases := map[string]capabilities{
		"v1.20.0": {MajorVersion: 1},
		"v1.25.0": {MinorVersion: 25},
		"v2.3.0":  {MajorVersion: "2", MinorVersion: "3"},
	}
	for want, caps := range cases {
		if got := unittestContext(nil, []capabilities{caps}, nil).KubeVersion; got != want {
			t.Errorf("%+v: KubeVersion = %q, want %q", caps, got, want)
		}
	}
	if got := unittestContext(nil, nil, nil).KubeVersion; got != "" {
		t.Errorf("no capabilities: KubeVersion = %q, want empty so the caller's default applies", got)
	}
}

func TestBuildCapabilitiesUsesGivenAPIVersions(t *testing.T) {
	t.Parallel()

	caps := BuildCapabilities("1.29.0", APIVersionSet{"custom/v1"})
	if got := caps["APIVersions"]; !reflect.DeepEqual(got, APIVersionSet{"custom/v1"}) {
		t.Errorf("APIVersions = %v, want only custom/v1", got)
	}
}

func TestTemplateGlobSkipsDirectoriesPartialsAndOtherFiles(t *testing.T) {
	t.Parallel()

	chart := t.TempDir()
	for _, f := range []string{"a.yaml", "sub/s.yaml", "_helpers.tpl", "README.md", "NOTES.txt"} {
		writeFile(t, filepath.Join(chart, "templates", f), "x")
	}
	got, err := expandTemplates(chart, []string{"templates/*", "templates/a.yam?"})
	if err != nil {
		t.Fatal(err)
	}
	// helm-unittest treats ? literally, so that entry is not a glob and stays as written.
	if want := []string{"templates/NOTES.txt", "templates/a.yaml", "templates/sub/s.yaml", "templates/a.yam?"}; !reflect.DeepEqual(got, want) {
		t.Errorf("expandTemplates = %v, want %v", got, want)
	}
}

func TestTemplateGlobWithoutTemplatesDirectoryFails(t *testing.T) {
	t.Parallel()

	if _, err := expandTemplates(t.TempDir(), []string{"*.yaml"}); err == nil {
		t.Fatal("want an error when the chart has no templates directory")
	}
}
