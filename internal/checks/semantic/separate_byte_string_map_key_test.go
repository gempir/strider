package semantic

import "testing"

func TestSeparateByteStringMapKeyReportsLookupOnlyTemporary(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

func allocated(items map[string]int, bytes []byte) int {
	key := string(bytes)
	return items[key] + items[key]
}

func direct(items map[string]int, bytes []byte) int {
	return items[string(bytes)]
}

func escaped(items map[string]int, bytes []byte) string {
	key := string(bytes)
	_ = items[key]
	return key
}
`,
	)
	registry, err := newRegistry([]string{
		"separate-byte-string-map-key",
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
