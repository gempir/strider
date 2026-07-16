package analyze_cases

import "math"

func properNaNTest(value float64) bool {
	return math.IsNaN(value)
}

func produceNaN(value float64) float64 {
	return value + math.NaN()
}
