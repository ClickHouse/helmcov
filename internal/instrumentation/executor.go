package instrumentation

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"text/template"
	"text/template/parse"
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
	for path, content := range sharedTemplates {
		sources[path] = content
	}
	sources[templateName] = tpl

	root := template.New("gotpl").Option("missingkey=zero")
	bindHelmTemplateFuncs(root, &trace)
	for _, path := range sortedKeys(sources) {
		if _, err := root.New(path).Parse(sources[path]); err != nil {
			return trace, "", fmt.Errorf("parse template %s: %w", path, err)
		}
	}

	parsed := root.Lookup(templateName)
	if parsed == nil {
		return trace, "", fmt.Errorf("template %s not found after parse", templateName)
	}

	registerTemplateLines(templateName, tpl, trace.Lines)

	var rendered bytes.Buffer
	if err := parsed.Execute(&rendered, values); err != nil {
		return trace, "", fmt.Errorf("execute template %s: %w", templateName, err)
	}

	walkTree(parsed.Root, templateName, root, values, tpl, &trace)
	traceDefinedTemplates(root, sources, values, &trace)

	if !hasCoveredLines(trace.Lines) {
		for lineKey := range trace.Lines {
			trace.Lines[lineKey] = 1
		}
	}

	return trace, rendered.String(), nil
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func traceDefinedTemplates(parsed *template.Template, sources map[string]string, values map[string]any, trace *Trace) {
	seen := map[string]bool{}
	for _, tmpl := range parsed.Templates() {
		name := tmpl.Name()
		if name == "" || name == "gotpl" || seen[name] {
			continue
		}
		if _, isFileTemplate := sources[name]; isFileTemplate {
			continue
		}
		if parsed.Lookup(name) == nil || tmpl.Tree == nil {
			continue
		}
		seen[name] = true
		sourcePath, sourceContent := findDefineSource(name, sources)
		if sourcePath == "" {
			continue
		}
		registerTemplateLines(sourcePath, sourceContent, trace.Lines)
		walkTree(tmpl.Root, sourcePath, parsed, values, sourceContent, trace)
	}
}

func findDefineSource(defineName string, sources map[string]string) (string, string) {
	for path, content := range sources {
		scanned, err := template.New("scan").Funcs(baseHelmFuncMap()).Option("missingkey=zero").Parse(content)
		if err != nil {
			continue
		}
		for _, tmpl := range scanned.Templates() {
			if tmpl.Name() == defineName {
				return path, content
			}
		}
	}
	return "", ""
}

func registerTemplateLines(templateName string, tpl string, lines map[string]int) {
	inComment := false
	for index, line := range strings.Split(tpl, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !inComment && strings.Contains(trimmed, "{{/*") {
			inComment = true
		}
		if inComment {
			if strings.Contains(trimmed, "*/}}") {
				inComment = false
			}
			continue
		}
		key := fmt.Sprintf("%s:%d", templateName, index+1)
		if _, exists := lines[key]; exists {
			continue
		}
		lines[key] = 0
	}
}

func walkTree(node parse.Node, templateName string, parsed *template.Template, values map[string]any, tpl string, trace *Trace) {
	markNodeLineHit(node, templateName, tpl, trace)
	switch typed := node.(type) {
	case *parse.ListNode:
		for _, item := range typed.Nodes {
			walkTree(item, templateName, parsed, values, tpl, trace)
		}
	case *parse.IfNode:
		line := nodeLine(typed, tpl)
		ifTrueKey := fmt.Sprintf("%s:%d:if:true", templateName, line)
		ifFalseKey := fmt.Sprintf("%s:%d:if:false", templateName, line)
		if _, ok := trace.Branches[ifTrueKey]; !ok {
			trace.Branches[ifTrueKey] = 0
		}
		if _, ok := trace.Branches[ifFalseKey]; !ok {
			trace.Branches[ifFalseKey] = 0
		}
		hit, err := evalPipe(parsed, typed.Pipe.String(), values)
		if err != nil {
			return
		}
		if hit {
			trace.Branches[ifTrueKey]++
			walkTree(typed.List, templateName, parsed, values, tpl, trace)
			return
		}
		trace.Branches[ifFalseKey]++
		if typed.ElseList != nil {
			walkTree(typed.ElseList, templateName, parsed, values, tpl, trace)
		}
	case *parse.RangeNode:
		line := nodeLine(typed, tpl)
		rangeNonEmptyKey := fmt.Sprintf("%s:%d:range:non-empty", templateName, line)
		rangeEmptyKey := fmt.Sprintf("%s:%d:range:empty", templateName, line)
		if _, ok := trace.Branches[rangeNonEmptyKey]; !ok {
			trace.Branches[rangeNonEmptyKey] = 0
		}
		if _, ok := trace.Branches[rangeEmptyKey]; !ok {
			trace.Branches[rangeEmptyKey] = 0
		}
		iterable, err := evalRaw(parsed, typed.Pipe.String(), values)
		if err != nil {
			return
		}
		if isRangeIterable(iterable) {
			trace.Branches[rangeNonEmptyKey]++
			walkTree(typed.List, templateName, parsed, values, tpl, trace)
			return
		}
		trace.Branches[rangeEmptyKey]++
		if typed.ElseList != nil {
			walkTree(typed.ElseList, templateName, parsed, values, tpl, trace)
		}
	case *parse.WithNode:
		line := nodeLine(typed, tpl)
		withNonEmptyKey := fmt.Sprintf("%s:%d:with:non-empty", templateName, line)
		withEmptyKey := fmt.Sprintf("%s:%d:with:empty", templateName, line)
		if _, ok := trace.Branches[withNonEmptyKey]; !ok {
			trace.Branches[withNonEmptyKey] = 0
		}
		if _, ok := trace.Branches[withEmptyKey]; !ok {
			trace.Branches[withEmptyKey] = 0
		}
		hit, err := evalPipe(parsed, typed.Pipe.String(), values)
		if err != nil {
			return
		}
		if hit {
			trace.Branches[withNonEmptyKey]++
			walkTree(typed.List, templateName, parsed, values, tpl, trace)
			return
		}
		trace.Branches[withEmptyKey]++
		if typed.ElseList != nil {
			walkTree(typed.ElseList, templateName, parsed, values, tpl, trace)
		}
	default:
		// no-op
	}
}

func markNodeLineHit(node parse.Node, templateName string, tpl string, trace *Trace) {
	if node == nil {
		return
	}
	pos := int(node.Position())
	if pos <= 0 || pos > len(tpl) {
		return
	}
	line := 1 + strings.Count(tpl[:pos-1], "\n")
	key := fmt.Sprintf("%s:%d", templateName, line)
	if _, ok := trace.Lines[key]; !ok {
		return
	}
	trace.Lines[key]++
}

func hasCoveredLines(lines map[string]int) bool {
	for _, hits := range lines {
		if hits > 0 {
			return true
		}
	}
	return false
}

func nodeLine(node parse.Node, tpl string) int {
	if node == nil {
		return 1
	}
	pos := int(node.Position())
	if pos <= 0 || pos > len(tpl) {
		return 1
	}
	return 1 + strings.Count(tpl[:pos-1], "\n")
}

func evalPipe(parsed *template.Template, pipe string, values map[string]any) (bool, error) {
	check := fmt.Sprintf("{{if %s}}true{{else}}false{{end}}", pipe)
	tpl, err := parsed.Clone()
	if err != nil {
		return false, err
	}
	tpl, err = tpl.New("eval").Parse(check)
	if err != nil {
		return false, err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, values); err != nil {
		return false, err
	}
	return strings.TrimSpace(buf.String()) == "true", nil
}

func evalRaw(parsed *template.Template, pipe string, values map[string]any) (any, error) {
	check := fmt.Sprintf("{{ $v := %s }}{{ printf \"%%#v\" $v }}", pipe)
	tpl, err := parsed.Clone()
	if err != nil {
		return nil, err
	}
	tpl, err = tpl.New("eval").Parse(check)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, values); err != nil {
		return nil, err
	}

	path := strings.TrimSpace(pipe)
	if strings.HasPrefix(path, ".") {
		if val, ok := lookupPath(values, path); ok {
			return val, nil
		}
	}
	return strings.TrimSpace(buf.String()), nil
}

func isRangeIterable(value any) bool {
	if value == nil {
		return false
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map, reflect.String:
		return v.Len() > 0
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return false
		}
		return isRangeIterable(v.Elem().Interface())
	default:
		return true
	}
}

func lookupPath(values map[string]any, path string) (any, bool) {
	parts := strings.Split(strings.TrimPrefix(path, "."), ".")
	var current any = values
	for _, part := range parts {
		asMap, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := asMap[part]
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}
