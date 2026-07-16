package analyze_cases

import "strings"

func trimValidUTF8(value string) string {
	return strings.Trim(value, "é")
}
