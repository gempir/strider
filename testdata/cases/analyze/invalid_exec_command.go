package analyze_cases

import "os/exec"

func runInvalidExecCommand() {
	exec.Command("go test")
}
