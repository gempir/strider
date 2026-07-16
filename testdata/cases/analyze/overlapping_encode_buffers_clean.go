package analyze_cases

import "encoding/hex"

func separateEncodeBuffers(destination, source []byte) int {
	return hex.Encode(destination, source)
}
