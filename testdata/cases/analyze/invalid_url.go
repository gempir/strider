package analyze_cases

import "net/url"

func parseInvalidURL() {
	url.Parse(":")
}
