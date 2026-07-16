package analyze_cases

func usedBeforeOverwrite() int {
	value := calculatedValue()
	useInteger(value)
	value = calculatedValue()
	return value
}
