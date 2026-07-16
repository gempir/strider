package analyze_cases

import "testing"

func TestFatalInTestGoroutine(t *testing.T) {
	if false {
		t.Fatal("failed")
	}
}
