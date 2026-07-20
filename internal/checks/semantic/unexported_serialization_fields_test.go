package semantic

import "testing"

func TestUnexportedSerializationFieldsReportsInvisibleData(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "encoding/json"

type private struct { value string }
type public struct { Value string }
type empty struct{}
type custom struct { value string }
func (custom) MarshalJSON() ([]byte, error) { return nil, nil }

func encode() {
	json.Marshal(private{})
	json.Marshal(public{})
	json.Marshal(empty{})
	json.Marshal(custom{})
	var destination private
	json.Unmarshal(nil, &destination)
}
`,
	)
	registry, err := newRegistry([]string{
		"unexported-serialization-fields",
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
