package analyze_cases

import "net/http"

func readCanonicalHeader(header http.Header) string {
	header["content-type"] = []string{"text/plain"}
	return header["Content-Type"][0]
}
