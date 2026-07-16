package analyze_cases

import "strings"

func trimInvalidUTF8(value string) string {
	return strings.Trim(value, "\xff")
}
