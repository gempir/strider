package analyze_cases

import "sort"

func sortNonSlice(values [3]int) {
	sort.Slice(values, func(left, right int) bool { return left < right })
}
