// Package instrumentation renders templates with Helm's function map and records line and branch hits.
package instrumentation

import (
	"bytes"
	"fmt"
	"maps"
	"slices"
	"strings"
	"text/template"
)

type Trace struct {
	Lines    map[string]int
	Branches map[string]int
}

// LookupFunc answers Helm's lookup function.
type LookupFunc func(apiVersion, kind, namespace, name string) (map[string]any, error)

// Executor renders templates. Lookup answers lookup; nil means no cluster, so
// lookup returns an empty map as it does under helm template.
type Executor struct {
	Lookup LookupFunc
}

func NewExecutor() *Executor {
	return &Executor{}
}

func (e *Executor) RenderAndTrace(templateName string, tpl string, values map[string]any, sharedTemplates map[string]string) (Trace, string, error) {
	trace := Trace{
		Lines:    map[string]int{},
		Branches: map[string]int{},
	}

	sources := map[string]string{}
	maps.Copy(sources, sharedTemplates)
	sources[templateName] = tpl

	root := template.New("gotpl").Option("missingkey=zero")
	bindHelmTemplateFuncs(root, &trace, e.Lookup)
	for _, path := range slices.Sorted(maps.Keys(sources)) {
		if _, err := root.New(path).Parse(sources[path]); err != nil {
			return trace, "", fmt.Errorf("parse template %s: %w", path, err)
		}
	}

	parsed := root.Lookup(templateName)
	if parsed == nil {
		return trace, "", fmt.Errorf("template %s not found after parse", templateName)
	}
	instrumentTemplates(root, templateName, sources, &trace)

	var rendered bytes.Buffer
	if err := parsed.Execute(&rendered, values); err != nil {
		return trace, "", fmt.Errorf("execute template %s: %w", templateName, err)
	}
	return trace, strings.ReplaceAll(rendered.String(), "<no value>", ""), nil
}
