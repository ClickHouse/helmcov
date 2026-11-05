package instrumentation

import (
	"bytes"
	"fmt"
	"maps"
	"slices"
	"text/template"
)

type Trace struct {
	Lines    map[string]int
	Branches map[string]int
}

type Executor struct{}

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
	bindHelmTemplateFuncs(root, &trace)
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
	return trace, rendered.String(), nil
}
