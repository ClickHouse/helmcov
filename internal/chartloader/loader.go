// Package chartloader reads charts, values, helm-unittest suites and the Helm render context.
package chartloader

import (
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
	sigsyaml "sigs.k8s.io/yaml"
)

type Bundle struct {
	Chart ChartMeta
	// Values is the chart's values.yaml, before any suite or test changes it.
	Values map[string]any
	Suites []Suite
}

type ChartMeta struct {
	Name       string `yaml:"name"`
	Version    string `yaml:"version"`
	AppVersion string `yaml:"appVersion"`
}

type Suite struct {
	Name         string             `yaml:"suite"`
	Templates    []string           `yaml:"templates"`
	ValuesFiles  []string           `yaml:"values"`
	Set          map[string]any     `yaml:"set"`
	Tests        []Test             `yaml:"tests"`
	Skip         map[string]any     `yaml:"skip"`
	Release      releaseSpec        `yaml:"release"`
	Capabilities capabilities       `yaml:"capabilities"`
	ChartMeta    chartOverride      `yaml:"chart"`
	Provider     kubernetesProvider `yaml:"kubernetesProvider"`
	Path         string
	MergedValues map[string]any
}

// Test is one helm-unittest test case. Its values and set layer on top of the suite's.
type Test struct {
	Name         string             `yaml:"it"`
	Template     string             `yaml:"template"`
	Templates    []string           `yaml:"templates"`
	ValuesFiles  []string           `yaml:"values"`
	Set          map[string]any     `yaml:"set"`
	Skip         map[string]any     `yaml:"skip"`
	Release      releaseSpec        `yaml:"release"`
	Capabilities capabilities       `yaml:"capabilities"`
	ChartMeta    chartOverride      `yaml:"chart"`
	Provider     kubernetesProvider `yaml:"kubernetesProvider"`
	MergedValues map[string]any     `yaml:"-"`
}

// TemplatesIn returns the templates the test renders: its own selection, or the suite's.
func (t Test) TemplatesIn(suite Suite) []string {
	if len(t.Templates) > 0 {
		return t.Templates
	}
	return suite.Templates
}

func DiscoverCharts(root string) ([]string, error) {
	var charts []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) != "Chart.yaml" {
			return nil
		}
		charts = append(charts, filepath.Dir(path))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover charts: %w", err)
	}
	sort.Strings(charts)
	return charts, nil
}

func DiscoverSuites(testsPath string) ([]string, error) {
	var suites []string

	err := filepath.WalkDir(testsPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		matched, err := filepath.Match("*_test.yaml", filepath.Base(path))
		if err != nil {
			return err
		}
		if matched {
			suites = append(suites, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover suites: %w", err)
	}
	sort.Strings(suites)
	return suites, nil
}

func LoadBundle(chartPath, testsPath string) (Bundle, error) {
	var bundle Bundle
	chartFile := filepath.Join(chartPath, "Chart.yaml")
	chartBytes, err := os.ReadFile(chartFile)
	if err != nil {
		return Bundle{}, fmt.Errorf("read Chart.yaml: %w", err)
	}
	if err := yaml.Unmarshal(chartBytes, &bundle.Chart); err != nil {
		return Bundle{}, fmt.Errorf("parse Chart.yaml: %w", err)
	}

	chartValues, err := loadYAMLMap(filepath.Join(chartPath, "values.yaml"))
	if err != nil && !os.IsNotExist(err) {
		return Bundle{}, fmt.Errorf("load chart values: %w", err)
	}
	if chartValues == nil {
		chartValues = map[string]any{}
	}
	bundle.Values = deepCopyMap(chartValues)

	suitePaths, err := DiscoverSuites(testsPath)
	if err != nil {
		return Bundle{}, err
	}

	for _, suitePath := range suitePaths {
		suiteBytes, readErr := os.ReadFile(suitePath)
		if readErr != nil {
			return Bundle{}, fmt.Errorf("read suite %s: %w", suitePath, readErr)
		}

		var suite Suite
		if unmarshalErr := yaml.Unmarshal(suiteBytes, &suite); unmarshalErr != nil {
			return Bundle{}, fmt.Errorf("parse suite %s: %w", suitePath, unmarshalErr)
		}
		// helm-unittest never runs skipped suites or tests, so their files need not exist.
		if suite.Skip != nil {
			continue
		}
		suite.Path = suitePath
		suiteDir := filepath.Dir(suitePath)
		if suite.Templates, err = expandTemplates(chartPath, suite.Templates); err != nil {
			return Bundle{}, fmt.Errorf("suite %s: %w", suitePath, err)
		}
		if suite.MergedValues, err = layerValues(chartValues, suiteDir, suite.ValuesFiles, suite.Set); err != nil {
			return Bundle{}, fmt.Errorf("load suite values %w", err)
		}
		tests := suite.Tests[:0]
		for _, test := range suite.Tests {
			if test.Skip != nil {
				continue
			}
			// As in helm-unittest: every values file first, then the suite's set, then the test's.
			files := append(slices.Clone(suite.ValuesFiles), test.ValuesFiles...)
			if test.MergedValues, err = layerValues(chartValues, suiteDir, files, suite.Set, test.Set); err != nil {
				return Bundle{}, fmt.Errorf("load values for test %q: %w", test.Name, err)
			}
			if test.Template != "" {
				test.Templates = append([]string{test.Template}, test.Templates...)
				test.Template = ""
			}
			if test.Templates, err = expandTemplates(chartPath, test.Templates); err != nil {
				return Bundle{}, fmt.Errorf("test %q: %w", test.Name, err)
			}
			tests = append(tests, test)
		}
		suite.Tests = tests

		bundle.Suites = append(bundle.Suites, suite)
	}

	return bundle, nil
}

// layerValues copies base, merges each values file (relative to dir, as helm-unittest
// resolves them), then each set in order, with its paths in key order.
func layerValues(base map[string]any, dir string, files []string, sets ...map[string]any) (map[string]any, error) {
	merged := deepCopyMap(base)
	for _, file := range files {
		if !filepath.IsAbs(file) {
			file = filepath.Join(dir, file)
		}
		values, err := loadYAMLMap(file)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		mergeInto(merged, values)
	}
	for _, set := range sets {
		for _, key := range slices.Sorted(maps.Keys(set)) {
			mergeInto(merged, setPathValue(key, set[key]))
		}
	}
	return merged, nil
}

// setPathValue turns a set path such as "a.list[1].b" into the nested value it
// describes, so merging it behaves like Helm's --set: maps merge, a list index
// replaces the whole list, and null deletes the key.
func setPathValue(path string, value any) map[string]any {
	parts := strings.Split(path, ".")
	for i := len(parts) - 1; i >= 0; i-- {
		name, indexes := splitIndexes(parts[i])
		for j := len(indexes) - 1; j >= 0; j-- {
			list := make([]any, indexes[j]+1)
			list[indexes[j]] = value
			value = list
		}
		value = map[string]any{name: value}
	}
	return value.(map[string]any)
}

var indexSuffix = regexp.MustCompile(`\[(\d+)\]$`)

// splitIndexes splits "list[1][0]" into "list" and [1, 0].
func splitIndexes(part string) (string, []int) {
	var indexes []int
	for {
		match := indexSuffix.FindStringSubmatchIndex(part)
		if match == nil {
			return part, indexes
		}
		index, _ := strconv.Atoi(part[match[2]:match[3]])
		indexes = append([]int{index}, indexes...)
		part = part[:match[0]]
	}
}

func loadYAMLMap(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return map[string]any{}, nil
	}
	// Helm reads values with sigs.k8s.io/yaml: YAML 1.1 booleans such as "off",
	// numbers as float64, and string keys only.
	var result map[string]any
	if err := sigsyaml.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if result == nil {
		return map[string]any{}, nil
	}
	return result, nil
}

func mergeInto(dst map[string]any, src map[string]any) {
	for key, value := range src {
		srcMap, srcIsMap := asStringMap(value)
		dstMap, dstIsMap := asStringMap(dst[key])
		switch {
		case value == nil:
			delete(dst, key) // Helm treats null as "remove this key"
		case srcIsMap && dstIsMap:
			mergeInto(dstMap, srcMap)
			dst[key] = dstMap
		case srcIsMap:
			dst[key] = deepCopyMap(srcMap)
		default:
			dst[key] = value
		}
	}
}

func deepCopyMap(src map[string]any) map[string]any {
	out := make(map[string]any, len(src))
	for key, value := range src {
		if child, ok := asStringMap(value); ok {
			out[key] = deepCopyMap(child)
		} else {
			out[key] = value
		}
	}
	return out
}

func asStringMap(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case map[any]any:
		// YAML decodes maps with non-string keys this way; Helm stringifies the keys.
		out := make(map[string]any, len(typed))
		for key, val := range typed {
			out[fmt.Sprint(key)] = val
		}
		return out, true
	default:
		return nil, false
	}
}
