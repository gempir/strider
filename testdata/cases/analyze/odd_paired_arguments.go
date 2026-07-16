package analyze_cases

import "strings"

func oddReplacementPairs() *strings.Replacer {
	return strings.NewReplacer("old", "new", "orphan")
}
