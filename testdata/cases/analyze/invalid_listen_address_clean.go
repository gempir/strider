package analyze_cases

import "net/http"

func validListenAddressExample(handler http.Handler) error {
	return http.ListenAndServe(":8080", handler)
}
