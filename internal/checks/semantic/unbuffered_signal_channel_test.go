package semantic

import "testing"

func TestUnbufferedSignalChannelReportsDirectUnbufferedChannels(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import (
	"os"
	"os/signal"
)

func configure() {
	unbuffered := make(chan os.Signal)
	buffered := make(chan os.Signal, 1)
	signal.Notify(unbuffered, os.Interrupt)
	signal.Notify(buffered, os.Interrupt)
}
`,
	)
	registry, err := newRegistry([]string{
		"unbuffered-signal-channel",
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
