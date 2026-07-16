package analyze_cases

import "strings"

func zeroReplacementLimit(value string) string {
	return strings.Replace(value, "old", "new", 0)
}
