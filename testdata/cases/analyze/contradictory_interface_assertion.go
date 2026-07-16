package analyze_cases

type integerReader interface {
	Read() int
}

type stringReader interface {
	Read() string
}

func impossibleInterfaceAssertion(value integerReader) {
	_, _ = value.(stringReader)
}
