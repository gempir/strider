package analyze_cases

import "strings"

func allocatingCaseComparison(left, right string) bool {
	return strings.ToLower(left) == strings.ToLower(right)
}
