package semantic

import "testing"

func TestWriterBufferMutationReportsStoreAndAppend(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

type writer struct{}

func (*writer) Write(buffer []byte) (int, error) {
	buffer[0] = 0
	buffer = append(buffer, 1)
	return len(buffer), nil
}

func (*writer) Other(buffer []byte) {
	buffer[0] = 0
}
`,
	)
	registry, err := newRegistry([]string{
		"writer-buffer-mutation",
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
