package chartloader

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func LoadTemplateFiles(chartPath string) (map[string]string, error) {
	templatesDir := filepath.Join(chartPath, "templates")
	info, err := os.Stat(templatesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("stat templates dir: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("templates path is not a directory: %s", templatesDir)
	}

	files := map[string]string{}
	err = filepath.WalkDir(templatesDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !isTemplateFile(path) {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read template %s: %w", path, readErr)
		}
		files[path] = string(content)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("load template files: %w", err)
	}
	return files, nil
}

func SortedTemplatePaths(files map[string]string) []string {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func isTemplateFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml", ".tpl", ".txt":
		return true
	default:
		return false
	}
}

func ResolveTemplatePath(chartPath, templatePath string) (string, error) {
	if filepath.IsAbs(templatePath) {
		if _, err := os.Stat(templatePath); err != nil {
			return "", fmt.Errorf("template not found: %s", templatePath)
		}
		return templatePath, nil
	}

	candidates := []string{
		filepath.Join(chartPath, templatePath),
		filepath.Join(chartPath, "templates", templatePath),
	}
	if trimmed := strings.TrimPrefix(templatePath, "templates/"); trimmed != templatePath {
		candidates = append(candidates, filepath.Join(chartPath, "templates", trimmed))
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("template not found: %s", templatePath)
}
