package valuegen

import (
	"reflect"
	"testing"
)

func TestGenerateCreatesBranchVariants(t *testing.T) {
	t.Parallel()

	base := map[string]any{
		"featureEnabled": true,
		"items":          []any{"a"},
	}

	scenarios := Generate(base, Options{MaxScenarios: 10, Seed: 42})
	if len(scenarios) < 3 {
		t.Fatalf("expected at least 3 scenarios, got %d", len(scenarios))
	}

	foundFalse := false
	foundEmptyItems := false
	for _, scenario := range scenarios {
		if enabled, ok := scenario["featureEnabled"].(bool); ok && !enabled {
			foundFalse = true
		}
		if items, ok := scenario["items"].([]any); ok && len(items) == 0 {
			foundEmptyItems = true
		}
	}

	if !foundFalse {
		t.Fatalf("expected a scenario with featureEnabled=false")
	}
	if !foundEmptyItems {
		t.Fatalf("expected a scenario with items empty")
	}
}

func TestGenerateDeterministicBySeed(t *testing.T) {
	t.Parallel()

	base := map[string]any{
		"featureEnabled": true,
		"items":          []any{"a"},
		"labels":         map[string]any{"app": "demo"},
	}

	left := Generate(base, Options{MaxScenarios: 8, Seed: 99})
	right := Generate(base, Options{MaxScenarios: 8, Seed: 99})
	if !reflect.DeepEqual(left, right) {
		t.Fatalf("expected deterministic scenarios for same seed")
	}
}

func TestGenerateHonorsMaxScenarios(t *testing.T) {
	t.Parallel()

	base := map[string]any{
		"featureEnabled": true,
		"items":          []any{"a"},
		"labels":         map[string]any{"app": "demo"},
	}

	scenarios := Generate(base, Options{MaxScenarios: 2, Seed: 1})
	if len(scenarios) != 2 {
		t.Fatalf("expected 2 scenarios, got %d", len(scenarios))
	}
}
