package analyze_cases

import "os/exec"

func runValidExecCommand() {
	exec.Command("go", "test", "./...")
}
