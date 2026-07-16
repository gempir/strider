package analyze_cases

import "encoding/json"

func unmarshalIntoNonPointer(data []byte, value map[string]any) error {
	return json.Unmarshal(data, value)
}
