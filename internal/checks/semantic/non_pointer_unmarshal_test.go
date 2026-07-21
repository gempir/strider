package semantic

import "testing"

func TestNonPointerUnmarshalReportsNonPointerDestinations(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import (
	"encoding/json"
	"encoding/xml"
)

func check(dynamic any) {
	var value map[string]any
	var boxed any = value
	json.Unmarshal(nil, value)
	json.Unmarshal(nil, boxed)
	json.Unmarshal(nil, dynamic)
	json.Unmarshal(nil, &value)
	json.NewDecoder(nil).Decode(value)
	xml.Unmarshal(nil, value)
}
`,
	)
	registry, err := newRegistry([]string{
		"non-pointer-unmarshal",
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
