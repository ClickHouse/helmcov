package instrumentation

import (
	"reflect"
	"strings"
	"testing"
)

func TestRenderAndTraceRecordsValueConstructs(t *testing.T) {
	t.Parallel()

	values := map[string]any{"Values": map[string]any{
		"on": true, "off": false, "name": "x", "empty": "",
	}}
	cases := []struct {
		name string
		tpl  string
		want map[string]int
	}{
		{
			name: "and records each operand it evaluates",
			tpl:  "{{ if and .Values.on .Values.off .Values.name }}x{{ end }}\n",
			want: map[string]int{
				"t.yaml:1:and(1):true": 1, "t.yaml:1:and(1):false": 0,
				"t.yaml:1:and(2):true": 0, "t.yaml:1:and(2):false": 1,
				// and stops at the first false operand, so the third never runs.
				"t.yaml:1:and(3):true": 0, "t.yaml:1:and(3):false": 0,
			},
		},
		{
			name: "or records operands up to the first true one",
			tpl:  "{{ or .Values.empty .Values.name .Values.on }}\n",
			want: map[string]int{
				"t.yaml:1:or(1):false": 1, "t.yaml:1:or(2):true": 1,
				"t.yaml:1:or(3):true": 0, "t.yaml:1:or(3):false": 0,
			},
		},
		{
			name: "default records whether the fallback was used",
			tpl:  "{{ .Values.missing | default \"a\" }}\n{{ default \"b\" .Values.name }}\n",
			want: map[string]int{
				"t.yaml:1:default:fallback": 1, "t.yaml:1:default:value": 0,
				"t.yaml:2:default:fallback": 0, "t.yaml:2:default:value": 1,
			},
		},
		{
			name: "ternary records the condition",
			tpl:  "{{ ternary \"y\" \"n\" .Values.off }}\n",
			want: map[string]int{"t.yaml:1:ternary:true": 0, "t.yaml:1:ternary:false": 1},
		},
		{
			name: "coalesce records which argument was used",
			tpl:  "{{ coalesce .Values.empty .Values.name \"z\" }}\n",
			want: map[string]int{"t.yaml:1:coalesce(1):used": 0, "t.yaml:1:coalesce(2):used": 1, "t.yaml:1:coalesce(3):used": 0},
		},
		{
			name: "else if on one line has its own keys",
			tpl:  "{{ if .Values.off }}a{{ else if .Values.on }}b{{ end }}\n",
			want: map[string]int{
				"t.yaml:1:if:false": 1, "t.yaml:1:if:true": 0,
				"t.yaml:1:else-if:true": 1, "t.yaml:1:else-if:false": 0,
			},
		},
		{
			name: "else with on one line has its own keys",
			tpl:  "{{ with .Values.empty }}a{{ else with .Values.name }}b{{ end }}\n",
			want: map[string]int{
				"t.yaml:1:with:empty": 1, "t.yaml:1:with:non-empty": 0,
				"t.yaml:1:else-with:non-empty": 1, "t.yaml:1:else-with:empty": 0,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			trace, _, err := NewExecutor().RenderAndTrace("t.yaml", tc.tpl, values, nil)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			for key, want := range tc.want {
				got, ok := trace.Branches[key]
				if !ok {
					t.Errorf("branch %s not registered; have %v", key, trace.Branches)
					continue
				}
				if got != want {
					t.Errorf("branch %s = %d, want %d", key, got, want)
				}
			}
		})
	}
}

// Wrapping operands and value functions must not change what renders.
func TestValueConstructsRenderLikePlainTemplates(t *testing.T) {
	t.Parallel()

	values := map[string]any{"Values": map[string]any{"on": true, "n": 0, "list": []any{1}, "name": "x"}}
	templates := []string{
		"{{ and .Values.on .Values.missing }}|{{ or .Values.missing .Values.n }}|{{ or .Values.missing \"d\" }}",
		"{{ if eq (or .Values.missing .Values.name) \"x\" }}eq{{ end }}|{{ len (or .Values.missing .Values.list) }}",
		"{{ .Values.missing | default (dict \"a\" 1) | toJson }}|{{ default 5 .Values.n }}|{{ coalesce .Values.missing 0 \"c\" }}",
		"{{ ternary 1 2 .Values.on }}|{{ .Values.on | ternary \"y\" \"n\" }}|{{ and 1 nil }}",
	}
	for i, tpl := range templates {
		_, got, err := NewExecutor().RenderAndTrace("t.yaml", tpl, values, nil)
		if err != nil {
			t.Fatalf("template %d: render: %v", i, err)
		}
		if want := plainRender(t, tpl, "", values); got != want {
			t.Errorf("template %d rendered %q, plain render gave %q", i, got, want)
		}
	}
}

func TestValueConstructKeysMatchTheArguments(t *testing.T) {
	t.Parallel()

	values := map[string]any{"Values": map[string]any{"name": "x", "empty": ""}}
	tpl := "{{ default \"a\" }}\n{{ .Values.name | coalesce .Values.empty }}\n{{ coalesce .Values.empty \"z\" }}\n"
	trace, _, err := NewExecutor().RenderAndTrace("t.yaml", tpl, values, nil)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	want := map[string]int{
		// default with no value argument always falls back.
		"t.yaml:1:default:fallback": 1, "t.yaml:1:default:value": 0,
		// The piped value is coalesce's last argument.
		"t.yaml:2:coalesce(1):used": 0, "t.yaml:2:coalesce(2):used": 1,
		"t.yaml:3:coalesce(1):used": 0, "t.yaml:3:coalesce(2):used": 1,
	}
	if !reflect.DeepEqual(trace.Branches, want) {
		t.Errorf("branches = %v, want exactly %v", trace.Branches, want)
	}
}

func TestNestedBranchesUnderPlainElseKeepTheirName(t *testing.T) {
	t.Parallel()

	tpl := "{{- if .Values.off }}a{{- else -}}\n{{ if .Values.on }}b{{ end }}{{ end }}\n{{- range .Values.empty }}{{ else -}}\n{{ if .Values.on }}c{{ end }}{{ end }}\n"
	values := map[string]any{"Values": map[string]any{"on": true, "off": false, "empty": []any{}}}
	trace, _, err := NewExecutor().RenderAndTrace("t.yaml", tpl, values, nil)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for key := range trace.Branches {
		if strings.Contains(key, "else-") {
			t.Errorf("%s: a nested if under a plain else is not an else if", key)
		}
	}
	if trace.Branches["t.yaml:2:if:true"] != 1 || trace.Branches["t.yaml:4:if:true"] != 1 {
		t.Errorf("nested branches = %v", trace.Branches)
	}
}

func TestLiteralOperandsAreNotBranches(t *testing.T) {
	t.Parallel()

	trace, _, err := NewExecutor().RenderAndTrace("t.yaml", "{{ or .Values.x \"d\" 1 true }}\n", map[string]any{"Values": map[string]any{}}, nil)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	want := map[string]int{"t.yaml:1:or(1):true": 0, "t.yaml:1:or(1):false": 1}
	if !reflect.DeepEqual(trace.Branches, want) {
		t.Errorf("branches = %v, want %v", trace.Branches, want)
	}
}
