package analyze_cases

import "sort"

func sortConversionWithoutSort(values []int) {
	values = sort.IntSlice(values)
	useInteger(values[0])
}
