//go:build !(aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris)

package main

import "os"

func processPeakRSS(_ *os.ProcessState) int64 {
	return 0
}

func machineDetails() (string, uint64) {
	return "", 0
}
