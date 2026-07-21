package semantic

import "testing"

func TestAddressNilComparisonReportsFixedResult(t *testing.T) {
	root := analysisModule(t, `package sample

func impossible(value int, pointer *int) bool {
	bad := &value == nil
	allowed := &*pointer == nil
	return bad || allowed
}
`)
	registry, err := newRegistry([]string{
		"address-nil-comparison",
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
