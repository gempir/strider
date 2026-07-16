package analyze_cases

type preservingWriter struct{}

func (*preservingWriter) Write(buffer []byte) (int, error) {
	copyOfBuffer := append([]byte(nil), buffer...)
	return len(copyOfBuffer), nil
}
