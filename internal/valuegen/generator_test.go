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

func TestGenerateEmptyMapVariantDoesNotClobberNestedDefaults(t *testing.T) {
	t.Parallel()

	base := map[string]any{
		"clickhouse": map[string]any{
			"nativePort": 9000,
			"prometheus": map[string]any{
				"port": 9363,
			},
		},
	}

	scenarios := Generate(base, Options{MaxScenarios: 20, Seed: 42})
	for index, scenario := range scenarios {
		clickhouse, ok := scenario["clickhouse"].(map[string]any)
		if !ok {
			t.Fatalf("scenario %d missing clickhouse map", index)
		}
		if len(clickhouse) == 0 {
			t.Fatalf("scenario %d replaced clickhouse with empty map", index)
		}
		if clickhouse["nativePort"] == nil {
			t.Fatalf("scenario %d dropped clickhouse.nativePort", index)
		}
		prometheus, ok := clickhouse["prometheus"].(map[string]any)
		if !ok {
			t.Fatalf("scenario %d missing clickhouse.prometheus map", index)
		}
		if len(prometheus) == 0 {
			t.Fatalf("scenario %d replaced clickhouse.prometheus with empty map", index)
		}
		if prometheus["port"] == nil {
			t.Fatalf("scenario %d dropped clickhouse.prometheus.port", index)
		}
	}
}
