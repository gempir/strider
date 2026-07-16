package analyze_cases

import "net/http"

func readNonCanonicalHeader(header http.Header) string {
	return header["content-type"][0]
}
