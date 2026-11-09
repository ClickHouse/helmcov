package chartloader

import (
	"cmp"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
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
	// objects, registered and clusterScoped come from kubernetesProvider and answer Lookup.
	objects       []map[string]any
	registered    map[string]bool
	clusterScoped map[string]bool
}

type kubernetesProvider struct {
	Scheme map[string]struct {
		Namespaced *bool `yaml:"namespaced"`
	} `yaml:"scheme"`
	Objects []map[string]any `yaml:"objects"`
}

// withProviders adds each provider's objects in order and records the kinds
// its scheme registers, and which of those are cluster-scoped.
func (c UnittestContext) withProviders(providers ...kubernetesProvider) UnittestContext {
	for _, p := range providers {
		c.objects = append(c.objects, p.Objects...)
		for kind, entry := range p.Scheme {
			c.registered = setKey(c.registered, kind)
			if entry.Namespaced != nil && !*entry.Namespaced {
				c.clusterScoped = setKey(c.clusterScoped, kind)
			}
		}
	}
	return c
}

func setKey(set map[string]bool, key string) map[string]bool {
	if set == nil {
		set = map[string]bool{}
	}
	set[key] = true
	return set
}

// Lookup answers Helm's lookup from the kubernetesProvider objects, as
// helm-unittest's fake client does. Only kinds in the scheme are found: a named
// object or an empty map, or with an empty name a <Kind>List of every match
// sorted by namespace and name. An empty namespace lists all namespaces, and
// cluster-scoped kinds ignore the namespace. Results are copies, so a template
// that changes one cannot affect the next render.
func (c UnittestContext) Lookup(apiVersion, kind, namespace, name string) (map[string]any, error) {
	key := apiVersion + "/" + kind
	if !c.registered[key] {
		return map[string]any{}, nil
	}
	clusterScoped := c.clusterScoped[key]
	items := []any{}
	for _, obj := range c.objects {
		meta, _ := obj["metadata"].(map[string]any)
		objNamespace, _ := meta["namespace"].(string)
		if obj["apiVersion"] != apiVersion || obj["kind"] != kind {
			continue
		}
		if !clusterScoped && objNamespace != namespace && (name != "" || namespace != "") {
			continue
		}
		if name == "" {
			items = append(items, copyObject(obj))
		} else if meta["name"] == name {
			return copyObject(obj), nil
		}
	}
	if name != "" {
		return map[string]any{}, nil
	}
	slices.SortFunc(items, func(a, b any) int {
		return cmp.Compare(objectKey(a), objectKey(b))
	})
	return map[string]any{
		"apiVersion": apiVersion, "kind": kind + "List", "items": items,
		"metadata": map[string]any{"continue": "", "resourceVersion": ""},
	}, nil
}

func objectKey(obj any) string {
	meta, _ := obj.(map[string]any)["metadata"].(map[string]any)
	namespace, _ := meta["namespace"].(string)
	name, _ := meta["name"].(string)
	return namespace + "/" + name
}

// copyObject deep-copies an object through JSON.
func copyObject(obj map[string]any) map[string]any {
	data, err := json.Marshal(obj)
	if err != nil {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return map[string]any{}
	}
	return out
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
	ctx := unittestContext([]releaseSpec{s.Release}, []capabilities{s.Capabilities}, []chartOverride{s.ChartMeta}).withProviders(s.Provider)
	items, _ := s.Capabilities.apiVersions()
	ctx.APIVersions = append(APIVersionSet{}, items...)
	return ctx
}

// ContextIn is the test's render context. As in helm-unittest, a field the test
// sets to a non-zero value overrides the suite's, and its API versions come
// before the suite's unless it sets apiVersions to null.
func (t Test) ContextIn(suite Suite) UnittestContext {
	ctx := unittestContext([]releaseSpec{suite.Release, t.Release}, []capabilities{suite.Capabilities, t.Capabilities}, []chartOverride{suite.ChartMeta, t.ChartMeta}).
		withProviders(suite.Provider, t.Provider)
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
			// Only a charts/ pattern reaches subchart templates, so "**" keeps matching the chart's own.
			if strings.HasPrefix(path, "charts/") && !strings.HasPrefix(pattern, "charts") {
				continue
			}
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

// templatePaths lists the renderable templates of the chart and its unpacked
// subcharts as paths from the chart root, such as "charts/sub/templates/a.yaml", sorted.
func templatePaths(chartPath string) ([]string, error) {
	roots := []string{filepath.Join(chartPath, "templates")}
	subs, err := unpackedSubcharts(chartPath)
	if err != nil {
		return nil, err
	}
	for _, sub := range subs {
		roots = append(roots, filepath.Join(sub.Path, "templates"))
	}
	var paths []string
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() || strings.HasPrefix(entry.Name(), "_") || !isTemplateFile(path) {
				return err
			}
			rel, err := filepath.Rel(chartPath, path)
			paths = append(paths, filepath.ToSlash(rel))
			return err
		})
		// No templates/ means no templates; a glob that then matches nothing still fails.
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	slices.Sort(paths)
	return paths, nil
}
