package analyze_cases

import "strings"

func uniqueTrimCutset(value string) string {
	return strings.TrimLeft(value, "abc")
}
