package analyze_cases

import "encoding/json"

type publicPayload struct {
	Value string
}

func visibleJSON() ([]byte, error) {
	return json.Marshal(publicPayload{Value: "visible"})
}
