package analyze_cases

import "io"

func currentAPIUsage(reader io.Reader) ([]byte, error) {
	return io.ReadAll(reader)
}
