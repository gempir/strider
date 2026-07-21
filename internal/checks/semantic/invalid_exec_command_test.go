package semantic

import "testing"

func TestInvalidExecCommandReportsShellCommandAsProgram(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "os/exec"

func check(dynamic string) {
	exec.Command("ls -la")
	exec.Command("ls", "-la")
	exec.Command("/Applications/My Program/tool")
	exec.Command(`+"`C:\\Program Files\\tool.exe`"+`)
	exec.Command(dynamic)
}
`,
	)
	registry, err := newRegistry([]string{
		"invalid-exec-command",
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
	if diagnostics[0].Code != "invalid-exec-command" {
		t.Fatalf("unexpected diagnostics: %#v", diagnostics)
	}
}
