package analyze_cases

import "net/url"

func parseValidURL(rawURL string) {
	url.Parse("https://golang.org")
	url.Parse(rawURL)
}
