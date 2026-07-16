package analyze_cases

func returnAfterNil(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
