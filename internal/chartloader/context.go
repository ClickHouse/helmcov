package chartloader

import "path/filepath"

type RenderOptions struct {
	Chart        ChartMeta
	Values       map[string]any
	ChartPath    string
	TemplatePath string
	KubeVersion  string
	Files        ChartFiles
}

func HelmRenderValues(opts RenderOptions) map[string]any {
	chartValues := map[string]any{
		"Name":    opts.Chart.Name,
		"Version": opts.Chart.Version,
	}
	if opts.Chart.AppVersion != "" {
		chartValues["AppVersion"] = opts.Chart.AppVersion
	}

	return map[string]any{
		"Values":       opts.Values,
		"Chart":        chartValues,
		"Release":      defaultReleaseValues(),
		"Capabilities": BuildCapabilities(opts.KubeVersion),
		"Files":        opts.Files,
		"Template":     templateContext(opts.ChartPath, opts.TemplatePath),
	}
}

func defaultReleaseValues() map[string]any {
	return map[string]any{
		"Name":      "release-name",
		"Namespace": "default",
		"Service":   "Helm",
		"Revision":  1,
		"IsInstall": true,
		"IsUpgrade": false,
	}
}

func templateContext(chartPath, templatePath string) map[string]any {
	name := filepath.Base(templatePath)
	basePath := ""

	if chartPath != "" && templatePath != "" {
		templatesRoot := filepath.Join(chartPath, "templates")
		if rel, err := filepath.Rel(templatesRoot, templatePath); err == nil {
			name = filepath.ToSlash(rel)
			dir := filepath.ToSlash(filepath.Dir(rel))
			if dir != "." {
				basePath = dir
			}
		}
	}

	return map[string]any{
		"Name":     name,
		"BasePath": basePath,
	}
}
