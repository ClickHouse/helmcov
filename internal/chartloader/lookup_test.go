package chartloader

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestLookupMatchesHelmUnittestKubernetesProvider(t *testing.T) {
	t.Parallel()

	chart := t.TempDir()
	writeFile(t, filepath.Join(chart, "Chart.yaml"), "apiVersion: v2\nname: demo\nversion: 0.1.0\n")
	writeFile(t, filepath.Join(chart, "tests", "s_test.yaml"), `suite: s
kubernetesProvider:
  scheme:
    "v1/Pod": {gvr: {version: v1, resource: pods}, namespaced: true}
    "v1/Namespace": {gvr: {version: v1, resource: namespaces}, namespaced: false}
  objects:
    - {kind: Pod, apiVersion: v1, metadata: {name: web, namespace: default}, spec: {nodeName: n1}}
    - {kind: Secret, apiVersion: v1, metadata: {name: unregistered, namespace: default}}
    - {kind: Pod, apiVersion: v1, metadata: {name: db, namespace: other}}
    - {kind: Namespace, apiVersion: v1, metadata: {name: ns1}}
tests:
  - it: adds its own objects
    kubernetesProvider:
      objects:
        - {kind: Pod, apiVersion: v1, metadata: {name: extra, namespace: default}}
`)
	bundle, err := LoadBundle(chart, filepath.Join(chart, "tests"))
	if err != nil {
		t.Fatal(err)
	}
	suite := bundle.Suites[0]
	test := suite.Tests[0].ContextIn(suite)

	web := map[string]any{"kind": "Pod", "apiVersion": "v1", "metadata": map[string]any{"name": "web", "namespace": "default"}, "spec": map[string]any{"nodeName": "n1"}}
	extra := map[string]any{"kind": "Pod", "apiVersion": "v1", "metadata": map[string]any{"name": "extra", "namespace": "default"}}
	podList := func(items ...any) map[string]any {
		return map[string]any{"apiVersion": "v1", "kind": "PodList", "items": items, "metadata": map[string]any{"continue": "", "resourceVersion": ""}}
	}
	cases := []struct {
		name                       string
		ctx                        UnittestContext
		apiVersion, kind, ns, item string
		want                       map[string]any
	}{
		{name: "named object", ctx: suite.Context(), apiVersion: "v1", kind: "Pod", ns: "default", item: "web", want: web},
		{name: "wrong namespace", ctx: suite.Context(), apiVersion: "v1", kind: "Pod", ns: "other", item: "web", want: map[string]any{}},
		{name: "missing object", ctx: suite.Context(), apiVersion: "v1", kind: "Pod", ns: "default", item: "nope", want: map[string]any{}},
		{name: "unknown kind", ctx: suite.Context(), apiVersion: "apps/v1", kind: "Deployment", ns: "default", item: "web", want: map[string]any{}},
		{name: "cluster-scoped kind ignores the namespace", ctx: suite.Context(), apiVersion: "v1", kind: "Namespace", ns: "default", item: "ns1",
			want: map[string]any{"kind": "Namespace", "apiVersion": "v1", "metadata": map[string]any{"name": "ns1"}}},
		{name: "list in a namespace", ctx: suite.Context(), apiVersion: "v1", kind: "Pod", ns: "default", want: podList(web)},
		// helm-unittest sorts list items by namespace, then name.
		{name: "test objects join the suite's, sorted", ctx: test, apiVersion: "v1", kind: "Pod", ns: "default", want: podList(extra, web)},
		// helm-unittest only finds kinds registered in the scheme.
		{name: "kind without a scheme entry", ctx: suite.Context(), apiVersion: "v1", kind: "Secret", ns: "default", item: "unregistered", want: map[string]any{}},
		{name: "list of a kind without a scheme entry", ctx: suite.Context(), apiVersion: "v1", kind: "Secret", ns: "default", want: map[string]any{}},
		{name: "list with no provider is falsy", ctx: UnittestContext{}, apiVersion: "v1", kind: "Pod", ns: "default", want: map[string]any{}},
	}
	for _, tc := range cases {
		got, err := tc.ctx.Lookup(tc.apiVersion, tc.kind, tc.ns, tc.item)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: lookup = %#v\nwant %#v", tc.name, got, tc.want)
		}
	}

	all, _ := suite.Context().Lookup("v1", "Pod", "", "")
	if items := all["items"].([]any); len(items) != 2 {
		t.Errorf("an empty namespace lists every namespace; got %d pods", len(items))
	}

	// A template that changes a lookup result must not change the next render's.
	first, _ := suite.Context().Lookup("v1", "Pod", "default", "web")
	first["metadata"].(map[string]any)["name"] = "changed"
	again, _ := suite.Context().Lookup("v1", "Pod", "default", "web")
	if again["metadata"].(map[string]any)["name"] != "web" {
		t.Error("lookup returned shared state")
	}
}
