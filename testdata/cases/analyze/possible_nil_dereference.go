package analyze_cases

func continueAfterNil(value *int, report func()) int {
	if value == nil {
		report()
	}
	return *value
}
