package analyze_cases

import "strconv"

func invalidStrconvArgument(value string) (int64, error) {
	return strconv.ParseInt(value, 1, 64)
}
