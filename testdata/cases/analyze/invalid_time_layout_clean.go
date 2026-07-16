package analyze_cases

import "time"

func parseValidTimeLayout(value string) {
	time.Parse("2006-01-02", value)
	time.Parse(time.RFC3339Nano, value)
}
