package analyze_cases

import "regexp"

func compiledRegexpMatch(values []string) {
	pattern := regexp.MustCompile("^[a-z]+$")
	for _, value := range values {
		pattern.MatchString(value)
	}
}
