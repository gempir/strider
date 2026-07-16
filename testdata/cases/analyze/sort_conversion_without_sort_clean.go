package analyze_cases

import "sort"

func sortWithHelper(values []int) {
	sort.Ints(values)
}

func sortWithInterface(values []int) {
	sort.Sort(sort.IntSlice(values))
}
