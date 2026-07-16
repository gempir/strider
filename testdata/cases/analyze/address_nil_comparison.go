package analyze_cases

func addressNilComparison(value int) bool {
	return &value == nil
}
