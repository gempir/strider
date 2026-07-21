package semantic

import "testing"

func TestDecimalFileModeReportsMissingOctalPrefix(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "os"

func write(path string, data []byte) {
	os.WriteFile(path, data, 644)
	os.WriteFile(path, data, 0o644)
	println(644)
}
`,
	)
	registry, err := newRegistry([]string{
		"decimal-file-mode",
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
