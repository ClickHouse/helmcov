package chartloader

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Bundle struct {
	Chart  ChartMeta
	Suites []Suite
}

type ChartMeta struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

type Suite struct {
	Name         string         `yaml:"suite"`
	Templates    []string       `yaml:"templates"`
	ValuesFiles  []string       `yaml:"values"`
	Set          map[string]any `yaml:"set"`
	Path         string
	MergedValues map[string]any
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
		suite.Path = suitePath
		suite.MergedValues = deepCopyMap(chartValues)

		for _, valueFile := range suite.ValuesFiles {
			abs := valueFile
			if !filepath.IsAbs(abs) {
				abs = filepath.Join(filepath.Dir(suitePath), valueFile)
			}
			valueMap, valueErr := loadYAMLMap(abs)
			if valueErr != nil {
				return Bundle{}, fmt.Errorf("load suite values %s: %w", abs, valueErr)
			}
			mergeInto(suite.MergedValues, valueMap)
		}
		for key, value := range suite.Set {
			setByPath(suite.MergedValues, key, value)
		}

		bundle.Suites = append(bundle.Suites, suite)
	}

	return bundle, nil
}

func loadYAMLMap(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return map[string]any{}, nil
	}
	var result map[string]any
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if result == nil {
		return map[string]any{}, nil
	}
	return result, nil
}

func mergeInto(dst map[string]any, src map[string]any) {
	for key, value := range src {
		existing, exists := dst[key]
		srcMap, srcIsMap := asStringMap(value)
		dstMap, dstIsMap := asStringMap(existing)
		if exists && srcIsMap && dstIsMap {
			mergeInto(dstMap, srcMap)
			dst[key] = dstMap
			continue
		}
		dst[key] = value
	}
}

func setByPath(dst map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	current := dst
	for idx, part := range parts {
		if idx == len(parts)-1 {
			current[part] = value
			return
		}

		next, ok := current[part]
		if !ok {
			newMap := map[string]any{}
			current[part] = newMap
			current = newMap
			continue
		}
		nextMap, nextMapOK := asStringMap(next)
		if !nextMapOK {
			newMap := map[string]any{}
			current[part] = newMap
			current = newMap
			continue
		}
		current = nextMap
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
		out := map[string]any{}
		for key, val := range typed {
			keyString, ok := key.(string)
			if !ok {
				return nil, false
			}
			out[keyString] = val
		}
		return out, true
	default:
		return nil, false
	}
}
