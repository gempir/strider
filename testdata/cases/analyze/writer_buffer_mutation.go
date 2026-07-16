package analyze_cases

type mutatingWriter struct{}

func (*mutatingWriter) Write(buffer []byte) (int, error) {
	buffer[0] = 0
	return len(buffer), nil
}
