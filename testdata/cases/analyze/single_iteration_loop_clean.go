package analyze_cases

func completeIteration(values []int) {
	for _, value := range values {
		if value < 0 {
			continue
		}
		useInteger(value)
	}
}

func arbitraryMapElement(values map[string]int) int {
	for _, value := range values {
		if value != 0 {
			useInteger(value)
		}
		return value
	}
	return 0
}
