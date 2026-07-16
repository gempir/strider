package analyze_cases

func changingAssignment(value, replacement int) int {
	value = replacement
	return value
}

func effectfulIndexAssignment(values []int, next func() int) {
	values[next()] = values[next()]
}
