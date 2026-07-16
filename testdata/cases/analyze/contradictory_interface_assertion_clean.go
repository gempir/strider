package analyze_cases

type readable interface {
	Read() int
}

type writable interface {
	Write() string
}

func possibleInterfaceAssertion(value readable) {
	_, _ = value.(writable)
}
