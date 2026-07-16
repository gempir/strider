package analyze_cases

import "math"

func nanComparison(value float64) bool {
	return value == math.NaN()
}
