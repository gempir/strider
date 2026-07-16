package analyze_cases

import "time"

const invalidTimeLayout = "12345"

func parseInvalidTimeLayout(value string) {
	time.Parse(invalidTimeLayout, value)
}
