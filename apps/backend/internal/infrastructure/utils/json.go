package utils

import (
	"encoding/json"
	"strings"
)

// UnmarshalFlexibleJSON unmarshals JSON with flexible key naming (camelCase or snake_case)
// It normalizes all keys to camelCase before unmarshaling into the target struct
func UnmarshalFlexibleJSON(data []byte, v interface{}) error {
	// First, unmarshal into a generic map
	var rawMap map[string]interface{}
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return err
	}

	// Normalize keys to camelCase
	normalizedMap := normalizeKeys(rawMap)

	// Marshal back to JSON
	normalizedJSON, err := json.Marshal(normalizedMap)
	if err != nil {
		return err
	}

	// Unmarshal into the target struct
	return json.Unmarshal(normalizedJSON, v)
}

// normalizeKeys recursively converts all map keys to camelCase
func normalizeKeys(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		normalized := make(map[string]interface{})
		for key, value := range v {
			// Convert snake_case to camelCase
			camelKey := snakeToCamel(key)
			normalized[camelKey] = normalizeKeys(value)
		}
		return normalized
	case []interface{}:
		// Recursively normalize array elements
		for i, item := range v {
			v[i] = normalizeKeys(item)
		}
		return v
	default:
		return data
	}
}

// snakeToCamel converts snake_case to camelCase
func snakeToCamel(s string) string {
	// If already camelCase, return as-is
	if !strings.Contains(s, "_") {
		return s
	}

	parts := strings.Split(s, "_")
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}
