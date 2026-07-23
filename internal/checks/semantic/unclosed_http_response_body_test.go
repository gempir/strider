package semantic

import "testing"

func TestUnclosedHTTPResponseBodyChecksDirectOwnership(t *testing.T) {
	reports := runResearchCorrectnessCheck(
		t,
		unclosedHTTPResponseBodyCheck{},
		`package fixture

import "net/http"

func missing(url string) error {
	response, err := http.Get(url)
	if err != nil { return err }
	_ = response.StatusCode
	return nil
}

func closed(url string) error {
	response, err := http.Get(url)
	if err != nil { return err }
	defer response.Body.Close()
	return nil
}

func transferred(url string) (*http.Response, error) {
	response, err := http.Get(url)
	if err != nil { return nil, err }
	return response, nil
}

func replaced() {
	response, _ := http.Get("first")
	response, _ = http.Get("second")
	defer response.Body.Close()
}
`,
	)
	assertResearchReportCount(t, reports, 2)
	assertResearchMessagesContain(t, reports, "HTTP response body")
}

func TestUnclosedHTTPResponseBodyLeavesAliasesToOwnershipAnalysis(t *testing.T) {
	reports := runResearchCorrectnessCheck(
		t,
		unclosedHTTPResponseBodyCheck{},
		`package fixture

import "net/http"

func alias(url string) error {
	response, err := http.Get(url)
	if err != nil { return err }
	body := response.Body
	defer body.Close()
	return nil
}
`,
	)
	assertResearchReportCount(t, reports, 0)
}

func TestUnclosedHTTPResponseBodyTracksNamedResultOwnership(t *testing.T) {
	reports := runResearchCorrectnessCheck(
		t,
		unclosedHTTPResponseBodyCheck{},
		`package fixture

import "net/http"

func transferred(url string) (response *http.Response, err error) {
	response, err = http.Get(url)
	return
}

func replaced(url string) (response *http.Response, err error) {
	response, err = http.Get(url + "/first")
	response, err = http.Get(url + "/second")
	return
}
`,
	)
	assertResearchReportCount(t, reports, 1)
	assertResearchMessagesContain(t, reports, "HTTP response body")
}
