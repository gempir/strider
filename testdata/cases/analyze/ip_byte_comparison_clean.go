package analyze_cases

import "net"

func ipValueComparison(left, right net.IP) bool {
	return left.Equal(right)
}
