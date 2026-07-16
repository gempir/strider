package analyze_cases

func observedAppendResult() []int {
	values := make([]int, 0, 1)
	values = append(values, 1)
	return values
}

func aliasedAppendStorage() {
	values := make([]int, 0, 1)
	alias := values
	values = append(values, 1)
	useSlice(alias)
	_ = values
}

func useSlice([]int) {}
