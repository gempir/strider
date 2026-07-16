package analyze_cases

type streamReader interface {
	Read([]byte) (int, error)
}

type streamReadCloser interface {
	streamReader
	Close() error
}

func unreachableTypeSwitchCase(value any) {
	switch value.(type) {
	case streamReader:
	case streamReadCloser:
	}
}
