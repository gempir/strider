package analyze_cases

import "net/url"

func persistURLQueryMutation(address *url.URL) {
	values := address.Query()
	values.Set("mode", "fast")
	address.RawQuery = values.Encode()
}
