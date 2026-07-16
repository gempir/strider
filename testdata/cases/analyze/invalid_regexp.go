package analyze_cases

import "regexp"

const invalidRegexp = "["

func compileInvalidRegexp() {
	regexp.MustCompile(invalidRegexp)
}
