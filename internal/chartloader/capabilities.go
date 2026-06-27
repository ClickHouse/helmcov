package chartloader

import "strings"

type APIVersionSet []string

func (set APIVersionSet) Has(apiVersion string) bool {
	for _, candidate := range set {
		if candidate == apiVersion {
			return true
		}
	}
	return false
}

func DefaultAPIVersions() APIVersionSet {
	return APIVersionSet{
		"v1",
		"apps/v1",
		"batch/v1",
		"autoscaling/v2",
		"networking.k8s.io/v1",
		"networking.k8s.io/v1/Ingress",
		"policy/v1",
		"policy/v1/PodDisruptionBudget",
		"rbac.authorization.k8s.io/v1",
		"storage.k8s.io/v1",
		"apiextensions.k8s.io/v1",
	}
}

func BuildCapabilities(kubeVersion string) map[string]any {
	version := normalizeKubeVersion(kubeVersion)
	major, minor := kubeMajorMinor(version)

	return map[string]any{
		"KubeVersion": map[string]any{
			"Version":      version,
			"Major":        major,
			"Minor":        minor,
			"GitVersion":   version,
			"GitCommit":    "helmcov",
			"GitTreeState": "clean",
			"GoVersion":    "go1.23",
			"Compiler":     "gc",
			"Platform":     "linux/amd64",
		},
		"APIVersions": DefaultAPIVersions(),
		"HelmVersion": map[string]any{
			"Version":      "v3.14.0",
			"GitCommit":    "helmcov",
			"GitTreeState": "clean",
			"GoVersion":    "go1.23",
		},
	}
}

func normalizeKubeVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return "v1.28.0"
	}
	if !strings.HasPrefix(version, "v") {
		return "v" + version
	}
	return version
}

func kubeMajorMinor(version string) (string, string) {
	trimmed := strings.TrimPrefix(version, "v")
	parts := strings.SplitN(trimmed, ".", 3)
	if len(parts) == 0 {
		return "1", "0"
	}
	major := parts[0]
	minor := "0"
	if len(parts) > 1 {
		minor = parts[1]
	}
	return major, minor
}
