package analyze_cases

import "regexp"

func repeatedRegexpMatch(values []string) {
	for _, value := range values {
		regexp.MatchString("^[a-z]+$", value)
	}
}
