package analyze_cases

import (
	"bytes"
	"net"
)

func ipByteComparison(left, right net.IP) bool {
	return bytes.Equal(left, right)
}
