//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

//strider:ignore-file cognitive-complexity,discarded-error-result,redundant-conversion,single-case-switch
package main

import (
	"bufio"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// processPeakRSS reads the kernel-maintained high-water mark for the child
// process. Darwin reports bytes; other supported Unix kernels report KiB.
func processPeakRSS(state *os.ProcessState) int64 {
	if state == nil {
		return 0
	}
	usage, ok := state.SysUsage().(*syscall.Rusage)
	if !ok {
		return 0
	}
	result := usage.Maxrss
	if runtime.GOOS != "darwin" {
		result *= 1024
	}
	return result
}

func machineDetails() (string, uint64) {
	switch runtime.GOOS {
	case "darwin":
		model, modelErr := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output()
		memory, memoryErr := exec.Command("sysctl", "-n", "hw.memsize").Output()
		bytes, parseErr := strconv.ParseUint(strings.TrimSpace(string(memory)), 10, 64)
		if modelErr != nil || memoryErr != nil || parseErr != nil {
			return runtime.GOARCH, 0
		}
		return strings.TrimSpace(string(model)), bytes
	case "linux":
		return linuxMachineDetails()
	default:
		return runtime.GOARCH, 0
	}
}

func linuxMachineDetails() (string, uint64) {
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return runtime.GOARCH, 0
	}
	defer file.Close()
	model := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, found := strings.Cut(scanner.Text(), ":")
		if found && strings.TrimSpace(key) == "model name" {
			model = strings.TrimSpace(value)
			break
		}
	}
	contents, readErr := os.ReadFile("/proc/meminfo")
	if readErr != nil {
		return model, 0
	}
	var memory uint64
	for _, line := range strings.Split(string(contents), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "MemTotal:" {
			kib, parseErr := strconv.ParseUint(fields[1], 10, 64)
			if parseErr != nil {
				return model, 0
			}
			memory = kib * 1024
			break
		}
	}
	return model, memory
}
