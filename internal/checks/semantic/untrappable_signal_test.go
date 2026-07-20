package semantic

import (
	"fmt"
	"runtime"
	"testing"
)

func TestUntrappableSignalReportsKernelHandledSignals(t *testing.T) {
	stopArgument := ", syscall.SIGSTOP"
	want := 4
	if runtime.GOOS == "windows" {
		stopArgument = ""
		want = 3
	}
	root := analysisModule(
		t,
		fmt.Sprintf(
			`package sample

import (
	"os"
	"os/signal"
	"syscall"
)

func configure(ch chan<- os.Signal) {
	signal.Notify(ch, os.Kill, syscall.SIGKILL%s)
	signal.Ignore(os.Signal(syscall.SIGKILL))
	signal.Reset(syscall.SIGTERM)
}
`,
			stopArgument,
		),
	)
	registry, err := newRegistry([]string{
		"untrappable-signal",
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
	if len(diagnostics) != want {
		t.Fatalf("got %d diagnostics, want %d: %#v", len(diagnostics), want, diagnostics)
	}
}
