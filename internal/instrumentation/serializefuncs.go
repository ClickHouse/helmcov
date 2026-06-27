package instrumentation

import (
	"bytes"
	"encoding/json"
	"strings"

	"gopkg.in/yaml.v3"
)

func fromYAML(str string) map[string]any {
	result := map[string]any{}
	if err := yaml.Unmarshal([]byte(str), &result); err != nil {
		result["Error"] = err.Error()
	}
	return result
}

func fromYAMLArray(str string) []any {
	result := []any{}
	if err := yaml.Unmarshal([]byte(str), &result); err != nil {
		return []any{err.Error()}
	}
	return result
}

func fromJSON(str string) map[string]any {
	result := map[string]any{}
	if err := json.Unmarshal([]byte(str), &result); err != nil {
		result["Error"] = err.Error()
	}
	return result
}

func fromJSONArray(str string) []any {
	result := []any{}
	if err := json.Unmarshal([]byte(str), &result); err != nil {
		return []any{err.Error()}
	}
	return result
}

func toJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}

func mustToJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func toYAMLPretty(value any) string {
	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	if err := encoder.Encode(value); err != nil {
		return ""
	}
	_ = encoder.Close()
	return strings.TrimSuffix(buffer.String(), "\n")
}
