package analyze_cases

import "regexp"

func compileValidRegexp() {
	regexp.MustCompile(`[a-z]+`)
}
