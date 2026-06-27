package instrumentation

import (
	"bytes"
	"errors"
	"fmt"
	"hash/fnv"
	"maps"
	"strconv"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"gopkg.in/yaml.v3"
)

const recursionMaxNums = 1000

func baseHelmFuncMap() template.FuncMap {
	funcMap := sprig.TxtFuncMap()
	delete(funcMap, "env")
	delete(funcMap, "expandenv")

	extras := template.FuncMap{
		"quote":         quote,
		"toYaml":        toYAML,
		"mustToYaml":    mustToYAML,
		"toYamlPretty":  toYAMLPretty,
		"fromYaml":      fromYAML,
		"fromYamlArray": fromYAMLArray,
		"toJson":        toJSON,
		"mustToJson":    mustToJSON,
		"fromJson":      fromJSON,
		"fromJsonArray": fromJSONArray,
		"include":       func(string, any) (string, error) { return "", fmt.Errorf("include not bound") },
		"tpl":           func(string, any) (string, error) { return "", fmt.Errorf("tpl not bound") },
		"required":      required,
		"fail":          fail,
		"lookup":        lookup,
	}
	maps.Copy(funcMap, extras)
	return funcMap
}

func bindHelmTemplateFuncs(t *template.Template, trace *Trace) {
	includedNames := make(map[string]int)
	funcMap := baseHelmFuncMap()
	funcMap["include"] = includeFun(t, includedNames)
	funcMap["tpl"] = tplFun(t, includedNames, trace)
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

		if trace != nil && parsed.Tree != nil {
			sourceName := tplSourceName(tpl)
			registerTemplateLines(sourceName, tpl, trace.Lines)
			valueMap, _ := vals.(map[string]any)
			walkTree(parsed.Root, sourceName, cloned, valueMap, tpl, trace)
		}

		var buf bytes.Buffer
		if err := parsed.Execute(&buf, vals); err != nil {
			return "", fmt.Errorf("error during tpl function execution for %q: %w", tpl, err)
		}
		return buf.String(), nil
	}
}

func tplSourceName(content string) string {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(content))
	return fmt.Sprintf("tpl:%x", hash.Sum32())
}

func quote(value any) string {
	return strconv.Quote(fmt.Sprintf("%v", value))
}

func toYAML(value any) string {
	data, err := yaml.Marshal(value)
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(string(data), "\n")
}

func mustToYAML(value any) string {
	data, err := yaml.Marshal(value)
	if err != nil {
		panic(err)
	}
	return strings.TrimSuffix(string(data), "\n")
}

func required(message string, value any) (any, error) {
	if value == nil {
		return nil, errors.New(message)
	}
	if typed, ok := value.(string); ok && typed == "" {
		return nil, errors.New(message)
	}
	return value, nil
}

func fail(message string) (string, error) {
	return "", errors.New(message)
}

func lookup(string, string, string, string) (map[string]any, error) {
	return map[string]any{}, nil
}
