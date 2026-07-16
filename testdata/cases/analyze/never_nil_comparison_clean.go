package analyze_cases

func possibleNilComparison() bool {
	var values []int
	if values == nil {
		return true
	}
	return false
}
