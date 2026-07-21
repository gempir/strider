package semantic

import "testing"

func TestSwappedSeekArgumentsReportsSwappedSeekArguments(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import (
	"io"
	"os"
)

type wrongSignature struct{}
func (wrongSignature) Seek(whence int, offset int64) (int64, error) { return 0, nil }

func check(seeker io.Seeker, file *os.File, custom wrongSignature) {
	seeker.Seek(io.SeekStart, 0)
	file.Seek(io.SeekEnd, 0)
	seeker.Seek(0, io.SeekStart)
	custom.Seek(io.SeekStart, 0)
}
`,
	)
	registry, err := newRegistry([]string{
		"swapped-seek-arguments",
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
