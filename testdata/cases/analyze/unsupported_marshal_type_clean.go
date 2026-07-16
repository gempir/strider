package analyze_cases

import "encoding/json"

type supportedPayload struct {
	Channel chan int `json:"-"`
}

func supportedMarshalType(value supportedPayload) ([]byte, error) {
	return json.Marshal(value)
}
