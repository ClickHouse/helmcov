package chartloader

import "testing"

func TestBuildCapabilitiesDefaultsKubeVersion(t *testing.T) {
	t.Parallel()

	caps := BuildCapabilities("")
	kube := caps["KubeVersion"].(map[string]any)
	if kube["GitVersion"] != "v1.28.0" {
		t.Fatalf("expected default git version, got %#v", kube["GitVersion"])
	}
	if kube["Major"] != "1" || kube["Minor"] != "28" {
		t.Fatalf("expected major/minor 1/28, got %#v %#v", kube["Major"], kube["Minor"])
	}
}

func TestBuildCapabilitiesNormalizesVersion(t *testing.T) {
	t.Parallel()

	caps := BuildCapabilities("1.30.2")
	kube := caps["KubeVersion"].(map[string]any)
	if kube["Version"] != "v1.30.2" {
		t.Fatalf("expected normalized version, got %#v", kube["Version"])
	}
}

func TestAPIVersionSetHas(t *testing.T) {
	t.Parallel()

	set := APIVersionSet{"batch/v1", "apps/v1"}
	if !set.Has("batch/v1") {
		t.Fatalf("expected batch/v1 to be present")
	}
	if set.Has("extensions/v1beta1") {
		t.Fatalf("expected extensions/v1beta1 to be absent")
	}
}
