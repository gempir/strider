package analyze_cases

import "testing"

func BenchmarkIterationMutation(b *testing.B) {
	b.N = 1000
}
