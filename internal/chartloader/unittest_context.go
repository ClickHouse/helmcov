package chartloader

import (
	"cmp"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// UnittestContext is the release, capabilities and chart metadata helm-unittest
// renders a suite or test with. Empty KubeVersion and chart fields mean the caller's default.
type UnittestContext struct {
	Release         map[string]any
	KubeVersion     string
	APIVersions     APIVersionSet
	ChartVersion    string
	ChartAppVersion string
}

type releaseSpec struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
	Revision  int    `yaml:"revision"`
	Upgrade   bool   `yaml:"upgrade"`
}

type capabilities struct {
	MajorVersion any `yaml:"majorVersion"`
	MinorVersion any `yaml:"minorVersion"`
	// A raw node, because helm-unittest treats a missing key, a list and null differently.
	APIVersions yaml.Node `yaml:"apiVersions"`
}

// apiVersions returns the list and whether apiVersions was explicitly null.
func (c capabilities) apiVersions() (items []string, null bool) {
	if c.APIVersions.Tag == "!!null" {
		return nil, true
	}
	_ = c.APIVersions.Decode(&items) // a missing key or a non-list means no versions
	return items, false
}

type chartOverride struct {
	Version    string `yaml:"version"`
	AppVersion string `yaml:"appVersion"`
}

// Context is the suite's render context on top of helm-unittest's defaults.
func (s Suite) Context() UnittestContext {
	ctx := unittestContext([]releaseSpec{s.Release}, []capabilities{s.Capabilities}, []chartOverride{s.ChartMeta})
	items, _ := s.Capabilities.apiVersions()
	ctx.APIVersions = append(APIVersionSet{}, items...)
	return ctx
}

// ContextIn is the test's render context. As in helm-unittest, a field the test
// sets to a non-zero value overrides the suite's, and its API versions come
// before the suite's unless it sets apiVersions to null.
func (t Test) ContextIn(suite Suite) UnittestContext {
	ctx := unittestContext([]releaseSpec{suite.Release, t.Release}, []capabilities{suite.Capabilities, t.Capabilities}, []chartOverride{suite.ChartMeta, t.ChartMeta})
	testItems, null := t.Capabilities.apiVersions()
	suiteItems, _ := suite.Capabilities.apiVersions()
	if null {
		suiteItems = nil
	}
	// Never nil: a nil set would mean helm template's defaults.
	ctx.APIVersions = append(append(APIVersionSet{}, testItems...), suiteItems...)
	return ctx
}

// unittestContext applies each layer over helm-unittest's defaults, which
// differ from helm template's: RELEASE-NAME, NAMESPACE, revision 0, no API versions.
func unittestContext(releases []releaseSpec, caps []capabilities, charts []chartOverride) UnittestContext {
	release := releaseSpec{Name: "RELEASE-NAME", Namespace: "NAMESPACE"}
	for _, l := range releases {
		release.Name = cmp.Or(l.Name, release.Name)
		release.Namespace = cmp.Or(l.Namespace, release.Namespace)
		release.Revision = cmp.Or(l.Revision, release.Revision)
		release.Upgrade = l.Upgrade || release.Upgrade
	}
	var major, minor any
	for _, l := range caps {
		major = cmpOrAny(l.MajorVersion, major)
		minor = cmpOrAny(l.MinorVersion, minor)
	}
	ctx := UnittestContext{
		Release: map[string]any{
			"Name": release.Name, "Namespace": release.Namespace, "Revision": release.Revision,
			"IsInstall": !release.Upgrade, "IsUpgrade": release.Upgrade, "Service": "Helm",
		},
		APIVersions: APIVersionSet{},
	}
	if major != nil || minor != nil {
		ctx.KubeVersion = fmt.Sprintf("v%v.%v.0", cmpOrAny(major, 1), cmpOrAny(minor, 20))
	}
	for _, c := range charts {
		ctx.ChartVersion = cmp.Or(c.Version, ctx.ChartVersion)
		ctx.ChartAppVersion = cmp.Or(c.AppVersion, ctx.ChartAppVersion)
	}
	return ctx
}

func cmpOrAny(value, fallback any) any {
	if value != nil {
		return value
	}
	return fallback
}

// expandTemplates expands glob entries the way helm-unittest does: only * is a
// wildcard, ** needs at least one character, "templates/" is prefixed unless the
// entry starts with templates, charts or **, and partials (_*) never match. A
// glob into charts/ that matches nothing is a packaged subchart and is skipped.
// Other entries are kept as written.
func expandTemplates(chartPath string, entries []string) ([]string, error) {
	var expanded []string
	var available []string
	for _, entry := range entries {
		if !strings.Contains(entry, "*") {
			expanded = appendUnique(expanded, entry)
			continue
		}
		if available == nil {
			var err error
			if available, err = templatePaths(chartPath); err != nil {
				return nil, err
			}
		}
		pattern := entry
		if !strings.HasPrefix(pattern, "templates") && !strings.HasPrefix(pattern, "charts") && !strings.HasPrefix(pattern, "**") {
			pattern = "templates/" + pattern
		}
		quoted := strings.ReplaceAll(regexp.QuoteMeta(pattern), `\*\*`, ".+")
		match := regexp.MustCompile("^" + strings.ReplaceAll(quoted, `\*`, ".*") + "$")
		matched := false
		for _, path := range available {
			if match.MatchString(path) {
				expanded = appendUnique(expanded, path)
				matched = true
			}
		}
		if !matched && !strings.HasPrefix(pattern, "charts") {
			return nil, fmt.Errorf("template glob %q matches no templates", entry)
		}
	}
	return expanded, nil
}

func appendUnique(list []string, item string) []string {
	if slices.Contains(list, item) {
		return list
	}
	return append(list, item)
}

// templatePaths lists the chart's renderable templates as "templates/..." paths, sorted.
func templatePaths(chartPath string) ([]string, error) {
	var paths []string
	root := filepath.Join(chartPath, "templates")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || strings.HasPrefix(entry.Name(), "_") || !isTemplateFile(path) {
			return err
		}
		rel, err := filepath.Rel(chartPath, path)
		paths = append(paths, filepath.ToSlash(rel))
		return err
	})
	slices.Sort(paths)
	return paths, err
}
