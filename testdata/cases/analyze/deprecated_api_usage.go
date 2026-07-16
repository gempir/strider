package analyze_cases

import (
	"io"
	"io/ioutil"
)

func deprecatedAPIUsage(reader io.Reader) ([]byte, error) {
	return ioutil.ReadAll(reader)
}
