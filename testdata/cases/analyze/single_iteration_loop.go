package analyze_cases

func singleIterationLoop(values []int) int {
	for _, value := range values {
		if value < 0 {
			useInteger(value)
		}
		return value
	}
	return 0
}

func useInteger(int) {}
