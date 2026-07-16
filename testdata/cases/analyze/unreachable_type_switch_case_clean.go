package analyze_cases

func reachableTypeSwitchCases(value any) {
	switch value.(type) {
	case streamReadCloser:
	case streamReader:
	case nil:
	}
}
