package analyze_cases

func optionalError(failed bool) error {
	if failed {
		return &typedProblem{}
	}
	return nil
}

func possibleInterfaceNilComparison(failed bool) bool {
	return optionalError(failed) == nil
}
