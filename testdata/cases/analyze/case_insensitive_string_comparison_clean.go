package analyze_cases

import "strings"

func foldedCaseComparison(left, right string) bool {
	return strings.EqualFold(left, right)
}
