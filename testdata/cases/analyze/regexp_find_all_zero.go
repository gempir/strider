package analyze_cases

import "regexp"

func findNoRegexpMatches(expression *regexp.Regexp, input string) []string {
	return expression.FindAllString(input, 0)
}
