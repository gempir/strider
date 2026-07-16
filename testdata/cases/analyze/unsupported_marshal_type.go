package analyze_cases

import "encoding/json"

func unsupportedMarshalType(ch chan int) ([]byte, error) {
	return json.Marshal(ch)
}
