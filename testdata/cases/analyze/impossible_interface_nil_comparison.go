package analyze_cases

type typedProblem struct{}

func (*typedProblem) Error() string { return "problem" }

func typedNilError() error {
	var problem *typedProblem
	return problem
}

func impossibleInterfaceNilComparison() bool {
	return typedNilError() == nil
}
