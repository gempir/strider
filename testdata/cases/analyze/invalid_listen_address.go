package analyze_cases

import "net/http"

func invalidListenAddress(handler http.Handler) error {
	return http.ListenAndServe("localhost", handler)
}
