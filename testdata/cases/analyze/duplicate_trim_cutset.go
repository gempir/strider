package analyze_cases

import "strings"

func duplicateTrimCutset(value string) string {
	return strings.TrimLeft(value, "letter")
}
