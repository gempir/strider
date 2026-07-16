package analyze_cases

func overwrittenBeforeUse() int {
	value := calculatedValue()
	value = calculatedValue()
	return value
}

func calculatedValue() int { return 1 }
