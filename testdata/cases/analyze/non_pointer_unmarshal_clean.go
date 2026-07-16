package analyze_cases

import "encoding/json"

func unmarshalIntoPointer(data []byte, value *map[string]any) error {
	return json.Unmarshal(data, value)
}
