package analyze_cases

import "sort"

func sortSlice(values []int) {
	sort.Slice(values, func(left, right int) bool { return values[left] < values[right] })
}
