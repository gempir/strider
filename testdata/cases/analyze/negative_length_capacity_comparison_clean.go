package analyze_cases

func validLengthCapacityComparison(values []int) bool {
	return len(values) == 0 || cap(values) > 0
}
