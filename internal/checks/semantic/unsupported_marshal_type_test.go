package semantic

import "testing"

func TestUnsupportedMarshalTypeReportsNestedUnsupportedValues(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import (
	"encoding/json"
	"encoding/xml"
)

type payload struct {
	Callback func()
	Ignored chan int `+"`json:\"-\" xml:\"-\"`"+`
}

type custom chan int

func (custom) MarshalJSON() ([]byte, error) { return []byte("null"), nil }

func encode(value payload, allowed custom) {
	json.Marshal(value)
	xml.Marshal(value)
	json.Marshal(allowed)
}
`,
	)
	registry, err := newRegistry([]string{
		"unsupported-marshal-type",
	})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{
		root,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	assertDiagnosticGolden(t, diagnostics)
}
