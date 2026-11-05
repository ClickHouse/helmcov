package instrumentation

import (
	"math"
	"strings"
	"testing"
)

func render(t *testing.T, tpl string, values map[string]any, helpers string) (string, error) {
	t.Helper()
	shared := map[string]string{}
	if helpers != "" {
		shared["_helpers.tpl"] = helpers
	}
	_, out, err := NewExecutor().RenderAndTrace("t.yaml", tpl, values, shared)
	return out, err
}

func TestHelmFunctions(t *testing.T) {
	t.Parallel()

	values := map[string]any{"Values": map[string]any{"name": "x", "nan": math.NaN()}}
	cases := []struct {
		name, tpl, want string
	}{
		{name: "required passes a set value through", tpl: `{{ required "need name" .Values.name }}`, want: "x"},
		{name: "required renders a missing value as empty", tpl: `[{{ required "need it" .Values.missing }}]`, want: "[]"},
		{name: "fail renders nothing", tpl: `[{{ fail "stop" }}]`, want: "[]"},
		{name: "lookup returns an empty map", tpl: `{{ len (lookup "v1" "Pod" "ns" "p") }}`, want: "0"},
		{name: "toJson of an unencodable value is empty", tpl: `[{{ toJson .Values.nan }}]`, want: "[]"},
		{name: "mustToYaml encodes", tpl: `{{ mustToYaml (dict "a" 1) }}`, want: "a: 1"},
		{name: "mustToJson encodes", tpl: `{{ mustToJson (dict "a" 1) }}`, want: `{"a":1}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := render(t, tc.tpl, values, "")
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if got != tc.want {
				t.Errorf("rendered %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMustSerializersFailOnUnencodableValues(t *testing.T) {
	t.Parallel()

	values := map[string]any{"Values": map[string]any{"ch": make(chan int), "nan": math.NaN()}}
	for _, tpl := range []string{"{{ mustToYaml .Values.ch }}", "{{ mustToJson .Values.nan }}"} {
		if _, err := render(t, tpl, values, ""); err == nil {
			t.Errorf("%s: want an error", tpl)
		}
	}
	if _, err := render(t, `{{ env "HOME" }}`, values, ""); err == nil {
		t.Error("env should not be defined")
	}
}

func TestIncludeRecursionLimit(t *testing.T) {
	t.Parallel()

	_, err := render(t, `{{ include "loop" . }}`, map[string]any{}, `{{- define "loop" }}{{ include "loop" . }}{{ end }}`)
	if err == nil || !strings.Contains(err.Error(), "nested reference name: loop") {
		t.Fatalf("err = %v, want the recursion limit error", err)
	}

	// The count drops after each include returns, so repeated sequential calls never hit the limit.
	out, err := render(t, `{{ range until 1500 }}{{ include "one" . }}{{ end }}`, map[string]any{}, `{{- define "one" }}1{{ end }}`)
	if err != nil {
		t.Fatalf("sequential includes: %v", err)
	}
	if len(out) != 1500 {
		t.Errorf("rendered %d includes, want 1500", len(out))
	}
}
