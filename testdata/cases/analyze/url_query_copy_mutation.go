package analyze_cases

import "net/url"

func urlQueryCopyMutation(address *url.URL) {
	address.Query().Add("mode", "fast")
}
