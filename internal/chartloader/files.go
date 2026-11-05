package chartloader

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"sigs.k8s.io/yaml"
)

// ChartFiles is the chart's .Files: every file except Chart.yaml, values files,
// templates/ and charts/, keyed by its path from the chart root. It is a map
// type, as in Helm, so templates can range over it.
type ChartFiles map[string][]byte

// Files Helm loads into other parts of the chart rather than .Files.
var specialChartFiles = map[string]bool{
	"Chart.yaml": true, "Chart.lock": true, "values.yaml": true, "values.schema.json": true,
	"requirements.yaml": true, "requirements.lock": true,
}

// LoadChartFiles reads .Files the way Helm does: it follows symlinks, strips a
// UTF-8 BOM and honours .helmignore.
func LoadChartFiles(chartPath string) (ChartFiles, error) {
	ignore, err := loadHelmignore(filepath.Join(chartPath, ".helmignore"))
	if err != nil {
		return nil, err
	}
	if ignore.ignores("Chart.yaml", false) {
		return nil, errors.New(".helmignore excludes Chart.yaml, so Helm cannot load this chart")
	}
	root, err := filepath.EvalSymlinks(chartPath)
	if err != nil {
		return nil, fmt.Errorf("resolve chart path: %w", err)
	}
	files := ChartFiles{}
	if err := files.load(chartPath, "", root, ignore, map[string]bool{}); err != nil {
		return nil, fmt.Errorf("load chart files: %w", err)
	}
	return files, nil
}

func insideRoot(link, root string) bool {
	target, err := filepath.EvalSymlinks(link)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, "../")
}

var utf8BOM = []byte("\xef\xbb\xbf")

func (f ChartFiles) load(dir, prefix, root string, ignore helmignore, seen map[string]bool) error {
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}
	// Only a directory that contains itself is a loop; other repeats are separate paths in Helm too.
	if seen[resolved] {
		return nil
	}
	seen[resolved] = true
	defer delete(seen, resolved)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		full := filepath.Join(dir, entry.Name())
		rel := path.Join(prefix, entry.Name())
		// Helm follows links anywhere; helmcov skips ones that leave the chart, so a chart
		// from an untrusted PR cannot read runner files such as credentials into .Files.
		if !insideRoot(full, root) {
			continue
		}
		info, err := os.Stat(full) // follows symlinks, as Helm does
		if err != nil {
			return err
		}
		if info.IsDir() {
			if rel == "templates" || rel == "charts" || ignore.ignores(rel, true) {
				continue
			}
			if err := f.load(full, rel, root, ignore, seen); err != nil {
				return err
			}
			continue
		}
		if specialChartFiles[rel] || ignore.ignores(rel, false) {
			continue
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file", full)
		}
		content, err := os.ReadFile(full)
		if err != nil {
			return err
		}
		f[rel] = bytes.TrimPrefix(content, utf8BOM)
	}
	return nil
}

// GetBytes returns a file's contents, or nothing when it does not exist.
func (f ChartFiles) GetBytes(name string) []byte {
	if content, ok := f[name]; ok {
		return content
	}
	return []byte{}
}

// Get returns a file's contents as a string, or "" when it does not exist.
func (f ChartFiles) Get(name string) string {
	return string(f.GetBytes(name))
}

// Glob returns the files matching pattern. An invalid pattern matches every file, as in Helm.
func (f ChartFiles) Glob(pattern string) ChartFiles {
	match, err := compileGlob(pattern)
	if err != nil {
		match = regexp.MustCompile(".*")
	}
	matched := ChartFiles{}
	for name, content := range f {
		if match.MatchString(name) {
			matched[name] = content
		}
	}
	return matched
}

// Lines splits a file into lines, without an empty line for a trailing newline.
func (f ChartFiles) Lines(name string) []string {
	content := string(f[name])
	if content == "" {
		return []string{}
	}
	return strings.Split(strings.TrimSuffix(content, "\n"), "\n")
}

// AsConfig renders the files as ConfigMap data, keyed by base name.
func (f ChartFiles) AsConfig() string {
	return f.asMap(func(content []byte) string { return string(content) })
}

// AsSecrets renders the files as Secret data: base64 values keyed by base name.
func (f ChartFiles) AsSecrets() string {
	return f.asMap(base64.StdEncoding.EncodeToString)
}

func (f ChartFiles) asMap(value func([]byte) string) string {
	if len(f) == 0 {
		return ""
	}
	m := make(map[string]string, len(f))
	for name, content := range f {
		m[path.Base(name)] = value(content)
	}
	data, err := yaml.Marshal(m)
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(string(data), "\n")
}

// compileGlob converts a Helm glob (gobwas/glob with '/' as the separator) to a
// regexp: ** crosses directories, * and ? do not, and {a,b} picks one alternative.
func compileGlob(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	depth := 0
	runes := []rune(pattern)
	for i := 0; i < len(runes); i++ {
		switch c := runes[i]; c {
		case '*':
			if i+1 < len(runes) && runes[i+1] == '*' {
				b.WriteString(".*")
				i++
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		case '{':
			depth++
			b.WriteString("(?:")
		case '}':
			if depth == 0 {
				return nil, fmt.Errorf("unmatched } in %q", pattern)
			}
			depth--
			b.WriteString(")")
		case ',':
			if depth > 0 {
				b.WriteString("|")
			} else {
				b.WriteString(",")
			}
		case '[':
			end := slices.Index(runes[i:], ']')
			if end < 0 {
				return nil, fmt.Errorf("unmatched [ in %q", pattern)
			}
			class := string(runes[i+1 : i+end])
			if strings.HasPrefix(class, "!") {
				class = "^" + class[1:]
			} else if strings.HasPrefix(class, "^") {
				class = `\^` + class[1:] // gobwas treats ^ literally
			}
			b.WriteString("[" + class + "]")
			i += end
		case '\\':
			if i+1 == len(runes) {
				return nil, fmt.Errorf("trailing backslash in %q", pattern)
			}
			i++
			b.WriteString(regexp.QuoteMeta(string(runes[i])))
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	if depth != 0 {
		return nil, fmt.Errorf("unmatched { in %q", pattern)
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

type helmignore []ignoreRule

type ignoreRule struct {
	pattern  string
	negate   bool
	dirOnly  bool
	fullPath bool
}

// loadHelmignore parses .helmignore with Helm's rules: ** and malformed globs
// are errors, a trailing / matches directories, a leading / or any / matches
// the path from the chart root, and other rules match the base name.
func loadHelmignore(file string) (helmignore, error) {
	data, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read .helmignore: %w", err)
	}
	var rules helmignore
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "**") {
			return nil, fmt.Errorf(".helmignore rule %q: ** is not supported by Helm", line)
		}
		if _, err := path.Match(line, "abc"); err != nil {
			return nil, fmt.Errorf(".helmignore rule %q: %w", line, err)
		}
		rule := ignoreRule{}
		if strings.HasPrefix(line, "!") {
			rule.negate = true
			line = line[1:]
		}
		if strings.HasSuffix(line, "/") {
			rule.dirOnly = true
			line = strings.TrimSuffix(line, "/")
		}
		rule.fullPath = strings.Contains(line, "/")
		rule.pattern = strings.TrimPrefix(line, "/")
		rules = append(rules, rule)
	}
	return rules, nil
}

// ignores copies Helm's Rules.Ignore, including how a negated rule ignores
// every path it does not match.
func (h helmignore) ignores(rel string, isDir bool) bool {
	for _, rule := range h {
		target := rel
		if !rule.fullPath {
			target = path.Base(rel)
		}
		matched, _ := path.Match(rule.pattern, target)
		if rule.negate {
			if (rule.dirOnly && !isDir) || !matched {
				return true
			}
			continue
		}
		if rule.dirOnly && !isDir {
			continue
		}
		if matched {
			return true
		}
	}
	return false
}
