package analyze_cases

func neverNilComparison() bool {
	values := make([]int, 0)
	if values == nil {
		return true
	}
	return false
}
