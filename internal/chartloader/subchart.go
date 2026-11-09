package chartloader

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Subchart is a dependency unpacked under a chart's charts/ directory.
type Subchart struct {
	Path   string
	Meta   ChartMeta
	Values map[string]any
}

// LoadSubchartFor returns the unpacked subchart that holds templatePath, written
// as charts/<subchart>/templates/<file>. ok is false for the parent's own templates.
// shortcut: nested subcharts render with the parent's context; support them when a chart needs it.
func LoadSubchartFor(chartPath, templatePath string) (Subchart, bool, error) {
	name, ok := subchartName(templatePath)
	if !ok {
		return Subchart{}, false, nil
	}
	return loadSubchart(filepath.Join(chartPath, "charts", name))
}

// loadSubchart reads the chart unpacked in dir. ok is false when dir holds no Chart.yaml.
func loadSubchart(dir string) (sub Subchart, ok bool, err error) {
	name := filepath.Base(dir)
	chartBytes, err := os.ReadFile(filepath.Join(dir, "Chart.yaml"))
	if os.IsNotExist(err) {
		return Subchart{}, false, nil
	}
	if err != nil {
		return Subchart{}, false, fmt.Errorf("read subchart %s Chart.yaml: %w", name, err)
	}
	sub.Path = dir
	if err := yaml.Unmarshal(chartBytes, &sub.Meta); err != nil {
		return Subchart{}, false, fmt.Errorf("parse subchart %s Chart.yaml: %w", name, err)
	}
	sub.Values, err = loadYAMLMap(filepath.Join(dir, "values.yaml"))
	if os.IsNotExist(err) {
		sub.Values = map[string]any{}
	} else if err != nil {
		return Subchart{}, false, fmt.Errorf("load subchart %s values: %w", name, err)
	}
	return sub, true, nil
}

// unpackedSubcharts loads every chart unpacked under chartPath/charts, sorted by directory.
func unpackedSubcharts(chartPath string) ([]Subchart, error) {
	entries, err := os.ReadDir(filepath.Join(chartPath, "charts"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read charts directory: %w", err)
	}
	var subs []Subchart
	for _, entry := range entries {
		dir := filepath.Join(chartPath, "charts", entry.Name())
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			continue // a packaged .tgz, or a broken link Helm would also fail to load
		}
		sub, ok, err := loadSubchart(dir)
		if err != nil {
			return nil, err
		}
		if ok {
			subs = append(subs, sub)
		}
	}
	return subs, nil
}

// coalesceSubchartDefaults puts each unpacked subchart's defaults under its name,
// as Helm does before any suite or test value applies; the parent's values win.
func coalesceSubchartDefaults(chartPath string, values map[string]any) error {
	subs, err := unpackedSubcharts(chartPath)
	if err != nil {
		return err
	}
	for _, sub := range subs {
		section := deepCopyMap(sub.Values)
		if own, ok := asStringMap(values[sub.Meta.Name]); ok {
			mergeInto(section, own)
		} else if values[sub.Meta.Name] != nil {
			continue // Helm keeps a non-map value and warns, so the defaults do not apply
		}
		values[sub.Meta.Name] = section
	}
	return nil
}

// ScopeValues returns the values Helm gives the subchart's templates: the parent's
// values under its name, which already hold its defaults, with the parent's globals on top.
func (s Subchart) ScopeValues(parent map[string]any) map[string]any {
	out := map[string]any{}
	if own, ok := asStringMap(parent[s.Meta.Name]); ok {
		out = deepCopyMap(own)
	}
	if globals, ok := asStringMap(parent["global"]); ok {
		merged, _ := asStringMap(out["global"])
		if merged == nil {
			merged = map[string]any{}
		}
		mergeInto(merged, globals)
		out["global"] = merged
	}
	return out
}
