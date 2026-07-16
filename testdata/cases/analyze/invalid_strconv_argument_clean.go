package analyze_cases

import "strconv"

func validStrconvArgument(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}
