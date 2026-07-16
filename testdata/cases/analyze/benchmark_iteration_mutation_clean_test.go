package analyze_cases

import "testing"

func BenchmarkIterations(b *testing.B) {
	for range b.N {
	}
}
