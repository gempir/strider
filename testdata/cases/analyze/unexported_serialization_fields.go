package analyze_cases

import "encoding/json"

type privatePayload struct {
	value string
}

func invisibleJSON() ([]byte, error) {
	return json.Marshal(privatePayload{value: "hidden"})
}
