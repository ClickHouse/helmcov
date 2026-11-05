package instrumentation

import (
	"bytes"
	"errors"
	"fmt"
	"hash/fnv"
	"maps"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"sigs.k8s.io/yaml"
)

const recursionMaxNums = 1000

// bindHelmTemplateFuncs gives t Helm's function map. Templates cannot read the
// environment. Unless strict, required and fail never stop a render, because
// generated scenarios often leave required values unset.
func bindHelmTemplateFuncs(t *template.Template, trace *Trace, lookup LookupFunc, strict bool) {
	if lookup == nil {
		lookup = noCluster
	}
	includedNames := make(map[string]int)
	funcMap := sprig.TxtFuncMap()
	delete(funcMap, "env")
	delete(funcMap, "expandenv")
	maps.Copy(funcMap, template.FuncMap{
		"toYaml":        toYAML,
		"mustToYaml":    mustToYAML,
		"toYamlPretty":  toYAMLPretty,
		"fromYaml":      fromYAML,
		"fromYamlArray": fromYAMLArray,
		"toJson":        toJSON,
		"mustToJson":    mustToJSON,
		"fromJson":      fromJSON,
		"fromJsonArray": fromJSONArray,
		"include":       includeFun(t, includedNames),
		"tpl":           tplFun(t, includedNames, trace),
		"required":      required,
		"fail":          func(string) string { return "" },
		"lookup":        lookup,
	})
	if strict {
		funcMap["fail"] = func(msg string) (string, error) { return "", errors.New(msg) }
		funcMap["required"] = strictRequired
	}
	maps.Copy(funcMap, coverageFuncs(trace))
	t.Funcs(funcMap)
}

func includeFun(t *template.Template, includedNames map[string]int) func(string, any) (string, error) {
	return func(name string, data any) (string, error) {
		var buf strings.Builder
		if count, ok := includedNames[name]; ok {
			if count >= recursionMaxNums {
				return "", fmt.Errorf("rendering template has a nested reference name: %s", name)
			}
			includedNames[name]++
		} else {
			includedNames[name] = 1
		}
		err := t.ExecuteTemplate(&buf, name, data)
		includedNames[name]--
		return buf.String(), err
	}
}

func tplFun(parent *template.Template, includedNames map[string]int, trace *Trace) func(string, any) (string, error) {
	return func(tpl string, vals any) (string, error) {
		cloned, err := parent.Clone()
		if err != nil {
			return "", fmt.Errorf("cannot clone template: %w", err)
		}
		cloned.Option("missingkey=zero")
		cloned.Funcs(template.FuncMap{
			"include": includeFun(cloned, includedNames),
			"tpl":     tplFun(cloned, includedNames, trace),
		})

		parsed, err := cloned.New(parent.Name()).Parse(tpl)
		if err != nil {
			return "", fmt.Errorf("cannot parse template %q: %w", tpl, err)
		}

		if parsed.Tree != nil {
			instrumentTree(parsed.Tree, tplSourceName(tpl), tpl, trace)
		}

		var buf bytes.Buffer
		if err := parsed.Execute(&buf, vals); err != nil {
			return "", fmt.Errorf("error during tpl function execution for %q: %w", tpl, err)
		}
		// Helm strips the marker text/template prints for a missing key.
		return strings.ReplaceAll(buf.String(), "<no value>", ""), nil
	}
}

func tplSourceName(content string) string {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(content))
	return fmt.Sprintf("tpl:%x", hash.Sum32())
}

func toYAML(value any) string {
	data, err := yaml.Marshal(value)
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(string(data), "\n")
}

func mustToYAML(value any) (string, error) {
	data, err := yaml.Marshal(value)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(data), "\n"), nil
}

func required(_ string, value any) any {
	if value == nil {
		return ""
	}
	return value
}

// strictRequired matches Helm's required: nil and empty strings are errors.
func strictRequired(msg string, value any) (any, error) {
	if value == nil {
		return nil, errors.New(msg)
	}
	if str, ok := value.(string); ok && str == "" {
		return nil, errors.New(msg)
	}
	return value, nil
}

func noCluster(string, string, string, string) (map[string]any, error) {
	return map[string]any{}, nil
}
