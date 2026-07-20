package semantic

import "testing"

func TestValidListenPortAcceptsHyphenatedServiceNames(t *testing.T) {
	for port, want := range map[string]bool{
		"http-alt":  true,
		"x11-1":     true,
		"-http":     false,
		"http-":     false,
		"http--alt": false,
		"123-456":   false,
	} {
		if got := validListenPort(port); got != want {
			t.Errorf("validListenPort(%q) = %t, want %t", port, got, want)
		}
	}
}
