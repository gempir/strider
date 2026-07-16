package analyze_cases

import "testing"

func TestFatalInGoroutine(t *testing.T) {
	go func() {
		t.Fatal("failed")
	}()
}
