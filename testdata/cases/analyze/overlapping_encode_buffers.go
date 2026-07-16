package analyze_cases

import "encoding/hex"

func overlappingEncodeBuffers(buffer []byte) int {
	return hex.Encode(buffer, buffer)
}
