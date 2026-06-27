package chartloader

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type ChartFiles struct {
	files map[string][]byte
}

func (f ChartFiles) WithFile(name string, content []byte) ChartFiles {
	cloned := map[string][]byte{}
	for key, value := range f.files {
		cloned[key] = value
	}
	cloned[name] = content
	return ChartFiles{files: cloned}
}

func LoadChartFiles(chartPath string) (ChartFiles, error) {
	filesDir := filepath.Join(chartPath, "files")
	info, err := os.Stat(filesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return ChartFiles{files: map[string][]byte{}}, nil
		}
		return ChartFiles{}, fmt.Errorf("stat chart files dir: %w", err)
	}
	if !info.IsDir() {
		return ChartFiles{}, fmt.Errorf("chart files path is not a directory: %s", filesDir)
	}

	loaded := map[string][]byte{}
	err = filepath.WalkDir(filesDir, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		content, readErr := os.ReadFile(filePath)
		if readErr != nil {
			return fmt.Errorf("read chart file %s: %w", filePath, readErr)
		}
		rel, relErr := filepath.Rel(filesDir, filePath)
		if relErr != nil {
			return relErr
		}
		loaded[filepath.ToSlash(rel)] = content
		return nil
	})
	if err != nil {
		return ChartFiles{}, fmt.Errorf("load chart files: %w", err)
	}

	return ChartFiles{files: loaded}, nil
}

func (f ChartFiles) Get(name string) (string, error) {
	content, err := f.GetBytes(name)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (f ChartFiles) GetBytes(name string) ([]byte, error) {
	key := normalizeChartFilePath(name)
	content, ok := f.files[key]
	if !ok {
		return nil, fmt.Errorf("files: %q not found", name)
	}
	cloned := make([]byte, len(content))
	copy(cloned, content)
	return cloned, nil
}

func (f ChartFiles) Glob(pattern string) ([]string, error) {
	matches := make([]string, 0)
	for name := range f.files {
		matched, err := matchChartFileGlob(pattern, name)
		if err != nil {
			return nil, err
		}
		if matched {
			matches = append(matches, name)
		}
	}
	sort.Strings(matches)
	return matches, nil
}

func (f ChartFiles) Lines(name string) ([]string, error) {
	content, err := f.Get(name)
	if err != nil {
		return nil, err
	}
	if content == "" {
		return []string{}, nil
	}
	return strings.Split(content, "\n"), nil
}

func (f ChartFiles) AsConfig() (string, error) {
	return f.renderFileBundle(false)
}

func (f ChartFiles) AsSecrets() (string, error) {
	return f.renderFileBundle(true)
}

func (f ChartFiles) renderFileBundle(secrets bool) (string, error) {
	if len(f.files) == 0 {
		return "", nil
	}

	names := make([]string, 0, len(f.files))
	for name := range f.files {
		names = append(names, name)
	}
	sort.Strings(names)

	var builder strings.Builder
	for _, name := range names {
		content := string(f.files[name])
		if secrets {
			fmt.Fprintf(&builder, "%s: %q\n", name, content)
			continue
		}
		fmt.Fprintf(&builder, "%s: |\n%s\n", name, indentLines(content, "  "))
	}
	return strings.TrimSuffix(builder.String(), "\n"), nil
}

func normalizeChartFilePath(name string) string {
	return filepath.ToSlash(strings.TrimPrefix(strings.ReplaceAll(name, "\\", "/"), "./"))
}

func matchChartFileGlob(pattern, name string) (bool, error) {
	pattern = normalizeChartFilePath(pattern)
	name = normalizeChartFilePath(name)

	if ok, err := path.Match(pattern, name); err != nil {
		return false, err
	} else if ok {
		return true, nil
	}

	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		return name == prefix || strings.HasPrefix(name, prefix+"/"), nil
	}
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		if name == prefix {
			return true, nil
		}
		if !strings.HasPrefix(name, prefix+"/") {
			return false, nil
		}
		rest := strings.TrimPrefix(name, prefix+"/")
		return !strings.Contains(rest, "/"), nil
	}

	return false, nil
}

func indentLines(content, prefix string) string {
	lines := strings.Split(content, "\n")
	for index, line := range lines {
		lines[index] = prefix + line
	}
	return strings.Join(lines, "\n")
}
