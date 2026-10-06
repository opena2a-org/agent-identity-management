package domain

import (
	"encoding/json"
	"fmt"
)

// DecodeTalksTo reads the agents.talks_to column as the entries its readers
// see. The column holds either an array of strings or an array of objects;
// an object stands for its "id", else its "name". An empty column, a JSON
// null and an empty array are all no entries.
func DecodeTalksTo(data []byte) ([]string, error) {
	if len(data) == 0 {
		return nil, nil
	}

	entries := make([]string, 0)
	if err := json.Unmarshal(data, &entries); err == nil {
		return entries, nil
	}

	var objects []map[string]interface{}
	if err := json.Unmarshal(data, &objects); err != nil {
		return nil, fmt.Errorf("failed to unmarshal talks_to: %w", err)
	}
	result := make([]string, 0, len(objects))
	for _, obj := range objects {
		if id, ok := obj["id"].(string); ok && id != "" {
			result = append(result, id)
		} else if name, ok := obj["name"].(string); ok && name != "" {
			result = append(result, name)
		}
	}
	return result, nil
}
