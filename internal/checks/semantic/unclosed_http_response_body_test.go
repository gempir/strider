package semantic

import "testing"

func TestUnclosedHTTPResponseBodyTracksCloseAndTransfer(t *testing.T) {
	reports := runResearchCorrectnessCheck(
		t,
		unclosedHTTPResponseBodyCheck{},
		`package fixture

import (
	"encoding/json"
	"errors"
	"net/http"
)

func bad(url string) error {
	response, err := http.Get(url)
	if err != nil { return err }
	_ = response.StatusCode
	return nil
}

func badDecode(url string) error {
	response, err := http.Get(url)
	if err != nil { return err }
	var value any
	return json.NewDecoder(response.Body).Decode(&value)
}

func good(url string) error {
	response, err := http.Get(url)
	if err != nil { return err }
	defer response.Body.Close()
	return nil
}

func goodBodyAlias(url string) error {
	response, err := http.Get(url)
	if err != nil { return err }
	body := response.Body
	defer body.Close()
	return nil
}

func goodDeferredWrapper(url string) error {
	response, err := http.Get(url)
	if err != nil { return err }
	defer func() { _ = response.Body.Close() }()
	return nil
}

func conditionalClose(url string, closeBody bool) error {
	response, err := http.Get(url)
	if err != nil { return err }
	if closeBody {
		defer response.Body.Close()
	}
	return nil
}

func conditionalDeferredWrapper(url string, closeBody bool) error {
	response, err := http.Get(url)
	if err != nil { return err }
	defer func() {
		if closeBody { _ = response.Body.Close() }
	}()
	return nil
}

func asynchronousClose(url string) error {
	response, err := http.Get(url)
	if err != nil { return err }
	go func() { _ = response.Body.Close() }()
	return nil
}

func reusedError(url string) error {
	response, err := http.Get(url)
	if err != nil { return err }
	err = errors.New("later failure")
	if err != nil { return err }
	return response.Body.Close()
}

func transferred(url string) (*http.Response, error) {
	response, err := http.Get(url)
	if err != nil { return nil, err }
	return response, nil
}

func sent(url string, output chan<- *http.Response) error {
	response, err := http.Get(url)
	if err != nil { return err }
	output <- response
	return nil
}
`,
	)
	assertResearchReportCount(t, reports, 6)
	assertResearchMessagesContain(t, reports, "HTTP response body")
}

func TestUnclosedHTTPResponseBodyTracksAliasGenerations(t *testing.T) {
	source := `package fixture

import "net/http"

func bodyAlias() {
	response, _ := http.Get("body-first")
	oldBody := response.Body
	response, _ = http.Get("body-second")
	defer oldBody.Close()
}

func responseAlias() {
	response, _ := http.Get("response-first")
	oldResponse := response
	response, _ = http.Get("response-second")
	defer oldResponse.Body.Close()
}

func bothClosed() {
	response, _ := http.Get("closed-first")
	oldResponse := response
	response, _ = http.Get("closed-second")
	defer oldResponse.Body.Close()
	defer response.Body.Close()
}

func pathDependent(useNew bool) {
	response, _ := http.Get("path-first")
	alias := response
	response, _ = http.Get("path-second")
	if useNew {
		alias = response
	}
	defer alias.Body.Close()
}
`
	reports := runResearchCorrectnessCheck(t, unclosedHTTPResponseBodyCheck{}, source)
	assertResearchReportCount(t, reports, 4)
	assertResearchReportNeedles(t, reports, source, "http.Get(\"body-second\")", "http.Get(\"response-second\")", "http.Get(\"path-first\")", "http.Get(\"path-second\")")
}

func TestUnclosedHTTPResponseBodyTracksBodyReplacementAndAliasTransfer(t *testing.T) {
	source := `package fixture

import (
	"io"
	"net/http"
	"strings"
)

func replaced() {
	response, _ := http.Get("replaced")
	response.Body = io.NopCloser(strings.NewReader("replacement"))
	defer response.Body.Close()
}

func transferredAlias() (io.ReadCloser, error) {
	response, err := http.Get("transferred")
	if err != nil { return nil, err }
	body := response.Body
	return body, nil
}

func conditionalDeferred(skip bool) {
	response, _ := http.Get("conditional-deferred")
	defer func() {
		if skip { return }
		_ = response.Body.Close()
	}()
}
`
	reports := runResearchCorrectnessCheck(t, unclosedHTTPResponseBodyCheck{}, source)
	assertResearchReportCount(t, reports, 2)
	assertResearchReportNeedles(t, reports, source, "http.Get(\"replaced\")", "http.Get(\"conditional-deferred\")")
}

func TestUnclosedHTTPResponseBodyTreatsNamedResultsAsTransfers(t *testing.T) {
	reports := runResearchCorrectnessCheck(
		t,
		unclosedHTTPResponseBodyCheck{},
		`package fixture

import (
	"io"
	"net/http"
)

func namedResponse() (response *http.Response, err error) {
	response, err = http.Get("named-response")
	return
}

func namedBody() (body io.ReadCloser, err error) {
	response, err := http.Get("named-body")
	if err != nil { return nil, err }
	body = response.Body
	return
}
`,
	)
	assertResearchReportCount(t, reports, 0)
}
