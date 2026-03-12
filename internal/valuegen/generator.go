package valuegen

import (
	"math/rand"
	"sort"
	"strings"
)

type Options struct {
	MaxScenarios int
	Seed         int64
}

func Generate(base map[string]any, options Options) []map[string]any {
	max := options.MaxScenarios
	if max <= 0 {
		max = 1
	}

	scenarios := []map[string]any{deepCopy(base)}
	variants := buildVariants(base)

	rng := rand.New(rand.NewSource(options.Seed))
	rng.Shuffle(len(variants), func(i, j int) {
		variants[i], variants[j] = variants[j], variants[i]
	})

	for _, variant := range variants {
		if len(scenarios) >= max {
			break
		}
		next := deepCopy(base)
		setPath(next, variant.path, variant.value)
		scenarios = append(scenarios, next)
	}

	if len(scenarios) > max {
		return scenarios[:max]
	}
	return scenarios
}

type variant struct {
	path  string
	value any
}

func buildVariants(base map[string]any) []variant {
	var result []variant
	collectVariants(base, "", &result)
	sort.Slice(result, func(i, j int) bool {
		return result[i].path < result[j].path
	})
	return result
}

func collectVariants(current map[string]any, prefix string, out *[]variant) {
	keys := make([]string, 0, len(current))
	for key := range current {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		value := current[key]
		switch typed := value.(type) {
		case bool:
			*out = append(*out, variant{path: path, value: !typed})
		case []any:
			if len(typed) == 0 {
				*out = append(*out, variant{path: path, value: []any{"generated"}})
			} else {
				*out = append(*out, variant{path: path, value: []any{}})
			}
		case []string:
			if len(typed) == 0 {
				*out = append(*out, variant{path: path, value: []any{"generated"}})
			} else {
				*out = append(*out, variant{path: path, value: []any{}})
			}
		case map[string]any:
			if len(typed) == 0 {
				*out = append(*out, variant{path: path, value: map[string]any{"generated": true}})
			} else {
				*out = append(*out, variant{path: path, value: map[string]any{}})
			}
			collectVariants(typed, path, out)
		}
	}
}

func deepCopy(src map[string]any) map[string]any {
	out := make(map[string]any, len(src))
	for key, value := range src {
		switch typed := value.(type) {
		case map[string]any:
			out[key] = deepCopy(typed)
		case []any:
			cloned := make([]any, len(typed))
			copy(cloned, typed)
			out[key] = cloned
		case []string:
			cloned := make([]any, len(typed))
			for idx, item := range typed {
				cloned[idx] = item
			}
			out[key] = cloned
		default:
			out[key] = value
		}
	}
	return out
}

func setPath(root map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	current := root
	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = value
			return
		}
		existing, ok := current[part]
		if !ok {
			next := map[string]any{}
			current[part] = next
			current = next
			continue
		}
		asMap, ok := existing.(map[string]any)
		if !ok {
			next := map[string]any{}
			current[part] = next
			current = next
			continue
		}
		current = asMap
	}
}
