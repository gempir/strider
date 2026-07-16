package analyze_cases

import "regexp"

func findAllRegexpMatches(expression *regexp.Regexp, input string) []string {
	return expression.FindAllString(input, -1)
}
