package chartloader

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("template not found: %s", templatePath)
}

// IsPackagedSubchartTemplate reports whether templatePath uses helm-unittest's subchart form,
// charts/<subchart>/templates/<file>, for a subchart that is not unpacked under chartPath.
func IsPackagedSubchartTemplate(chartPath, templatePath string) bool {
	name, ok := subchartName(templatePath)
	if !ok {
		return false
	}
	info, err := os.Stat(filepath.Join(chartPath, "charts", name))
	return err != nil || !info.IsDir()
}

// subchartName returns <subchart> from helm-unittest's charts/<subchart>/templates/<file> form.
func subchartName(templatePath string) (string, bool) {
	parts := strings.Split(filepath.ToSlash(templatePath), "/")
	if len(parts) < 4 || parts[0] != "charts" || parts[1] == "" || parts[2] != "templates" {
		return "", false
	}
	return parts[1], true
}
