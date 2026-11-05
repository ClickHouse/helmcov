package instrumentation

import (
	"strings"
	"testing"
	"text/template"
)

// These cases change dot, use variables or exit early, so a branch is only
// counted correctly if it is recorded while the template actually executes.
func TestRenderAndTraceRecordsBranchesInExecutionScope(t *testing.T) {
	t.Parallel()

	values := map[string]any{"Values": map[string]any{
		"flag":  true,
		"items": []any{map[string]any{"enabled": true}, map[string]any{"enabled": true}},
	}}

	cases := []struct {
		name    string
		tpl     string
		helpers string
		want    map[string]int
	}{
		{
			name: "if on dot inside range",
			tpl:  "{{- range .Values.items }}\n{{- if .enabled }}on{{ else }}off{{ end }}\n{{- end }}\n",
			want: map[string]int{"t.yaml:2:if:true": 2, "t.yaml:2:if:false": 0},
		},
		{
			name:    "if on include argument",
			tpl:     "{{ include \"x\" (index .Values.items 0) }}\n",
			helpers: "{{- define \"x\" }}\n{{- if .enabled }}on{{ else }}off{{ end }}\n{{- end }}\n",
			want:    map[string]int{"_helpers.tpl:2:if:true": 1, "_helpers.tpl:2:if:false": 0},
		},
		{
			name: "if on variable inside with",
			tpl:  "{{- $on := .Values.flag }}\n{{- with .Values.items }}\n{{- if $on }}on{{ end }}\n{{- end }}\n",
			want: map[string]int{"t.yaml:3:if:true": 1, "t.yaml:3:if:false": 0},
		},
		{
			name: "break leaves the range after the first item",
			tpl:  "{{- range .Values.items }}\n{{- if .enabled }}{{ break }}{{ end }}\n{{- end }}\n",
			want: map[string]int{"t.yaml:2:if:true": 1, "t.yaml:2:if:false": 0},
		},
		{
			name: "else if chain takes the second condition",
			tpl:  "{{- if not .Values.flag }}a\n{{- else if .Values.flag }}b\n{{- else }}c\n{{- end }}\n",
			want: map[string]int{"t.yaml:1:if:false": 1, "t.yaml:2:else-if:true": 1, "t.yaml:2:else-if:false": 0},
		},
		{
			name: "else with chain takes the second value",
			tpl:  "{{- with .Values.missing }}a\n{{- else with .Values.items }}b\n{{- end }}\n",
			want: map[string]int{"t.yaml:1:with:empty": 1, "t.yaml:2:else-with:non-empty": 1, "t.yaml:2:else-with:empty": 0},
		},
		{
			name: "continue skips the rest of each iteration",
			tpl:  "{{- range .Values.items }}\n{{- if .enabled }}{{ continue }}{{ end }}\n{{- if .enabled }}never{{ end }}\n{{- end }}\n",
			want: map[string]int{"t.yaml:2:if:true": 2, "t.yaml:3:if:true": 0, "t.yaml:3:if:false": 0},
		},
		{
			name: "range over a map with break",
			tpl:  "{{- range $k, $v := .Values }}\n{{- if eq $k \"flag\" }}{{ break }}{{ end }}\n{{- end }}\n",
			want: map[string]int{"t.yaml:1:range:non-empty": 1, "t.yaml:2:if:true": 1, "t.yaml:2:if:false": 0},
		},
		{
			name: "tpl of a missing value is empty, as in Helm",
			tpl:  "{{- if tpl \"{{ .Values.missing }}\" . }}x{{ end }}\n",
			want: map[string]int{"t.yaml:1:if:true": 0, "t.yaml:1:if:false": 1},
		},
		{
			name: "branches under an untaken branch are registered at zero",
			tpl:  "{{- if not .Values.flag }}\n{{- if .Values.other }}x{{ end }}\n{{- end }}\n",
			want: map[string]int{"t.yaml:2:if:true": 0, "t.yaml:2:if:false": 0},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			shared := map[string]string{}
			if tc.helpers != "" {
				shared["_helpers.tpl"] = tc.helpers
			}
			trace, _, err := NewExecutor().RenderAndTrace("t.yaml", tc.tpl, values, shared)
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

func TestRenderAndTraceCoversEveryLineOfExecutedText(t *testing.T) {
	t.Parallel()

	tpl := "apiVersion: v1\nkind: ConfigMap\ndata:\n  name: {{ .Values.name }}\n"
	values := map[string]any{"Values": map[string]any{"name": "x"}}

	trace, _, err := NewExecutor().RenderAndTrace("t.yaml", tpl, values, nil)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, key := range []string{"t.yaml:1", "t.yaml:2", "t.yaml:3", "t.yaml:4"} {
		if trace.Lines[key] == 0 {
			t.Errorf("line %s not covered; have %v", key, trace.Lines)
		}
	}
}

func TestRenderAndTraceAttributesTextAfterTrimMarkers(t *testing.T) {
	t.Parallel()

	// -}} trims the newline, so "b" starts on line 3 and must not be reported on line 2.
	tpl := "a: 1\n{{- if .Values.on -}}\n\nb: 2\n{{- end }}\n"
	trace, _, err := NewExecutor().RenderAndTrace("t.yaml", tpl, map[string]any{"Values": map[string]any{"on": true}}, nil)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	want := map[string]int{"t.yaml:1": 1, "t.yaml:2": 1, "t.yaml:4": 1}
	for key, hits := range want {
		if trace.Lines[key] != hits {
			t.Errorf("line %s = %d, want %d", key, trace.Lines[key], hits)
		}
	}
	if len(trace.Lines) != len(want) {
		t.Errorf("coverable lines = %v, want exactly %v", trace.Lines, want)
	}
}

func TestRenderAndTraceRecordsTplLines(t *testing.T) {
	t.Parallel()

	content := "first\n{{ if .Values.on }}second{{ end }}"
	tpl := "{{ tpl .Values.text . }}\n"
	values := map[string]any{"Values": map[string]any{"on": false, "text": content}}

	trace, _, err := NewExecutor().RenderAndTrace("t.yaml", tpl, values, nil)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	source := tplSourceName(content)
	if trace.Lines[source+":1"] != 1 {
		t.Errorf("%s:1 = %d, want 1", source, trace.Lines[source+":1"])
	}
	if trace.Branches[source+":2:if:false"] != 1 || trace.Branches[source+":2:if:true"] != 0 {
		t.Errorf("tpl branches = %v", trace.Branches)
	}
}

// The markers must never change what a chart renders.
func TestRenderAndTraceOutputMatchesPlainRender(t *testing.T) {
	t.Parallel()

	helpers := "{{- define \"x\" -}}\n{{- if .enabled }}on{{ else }}off{{ end -}}\n{{- end -}}\n"
	templates := []string{
		"a: {{ if .Values.flag }}yes{{ end }}\nb: {{ include \"x\" (index .Values.items 0) }}\n",
		"{{- range $i, $v := .Values.items }}\n- {{ $i }}{{ if $v.enabled }} on{{ else }} off{{ end }}\n{{- else }}\nnone\n{{- end }}\n",
		"{{- with .Values.missing }}x{{ else with .Values.flag }}{{ . }}{{ end }}\n{{ range 3 }}{{ if eq . 1 }}{{ continue }}{{ end }}{{ . }}{{ end }}\n",
	}
	values := map[string]any{"Values": map[string]any{
		"flag":  true,
		"items": []any{map[string]any{"enabled": true}, map[string]any{"enabled": false}},
	}}

	for i, tpl := range templates {
		_, got, err := NewExecutor().RenderAndTrace("t.yaml", tpl, values, map[string]string{"_helpers.tpl": helpers})
		if err != nil {
			t.Fatalf("template %d: render: %v", i, err)
		}
		want := plainRender(t, tpl, helpers, values)
		if got != want {
			t.Errorf("template %d rendered %q, plain render gave %q", i, got, want)
		}
	}
}

func plainRender(t *testing.T, tpl, helpers string, values map[string]any) string {
	t.Helper()
	root := template.New("gotpl").Option("missingkey=zero")
	bindHelmTemplateFuncs(root, &Trace{Lines: map[string]int{}, Branches: map[string]int{}}, nil)
	template.Must(root.New("_helpers.tpl").Parse(helpers))
	main := template.Must(root.New("t.yaml").Parse(tpl))
	var out strings.Builder
	if err := main.Execute(&out, values); err != nil {
		t.Fatalf("plain render: %v", err)
	}
	return strings.ReplaceAll(out.String(), "<no value>", "")
}
