package semantic

import "testing"

func TestSlicePreallocationConservativeCases(t *testing.T) {
	assertCheckDiagnostics(
		t,
		"slice-preallocation",
		`package sample

func collect(source []int) []int {
	var result []int
	for _, value := range source {
		result = append(result, value)
	}
	return result
}

func alreadySized(source []int) []int {
	result := make([]int, 0, len(source))
	for _, value := range source {
		result = append(result, value)
	}
	return result
}

func conditional(source []int) []int {
	var result []int
	for _, value := range source {
		if value > 0 {
			result = append(result, value)
		}
	}
	return result
}

func resetEachIteration(source []int) []int {
	var result []int
	for _, value := range source {
		result = nil
		result = append(result, value)
	}
	return result
}

func rangeOverResult() []int {
	var result []int
	for _, value := range result {
		result = append(result, value)
	}
	return result
}
`,
		1,
		"preallocate",
	)
}
