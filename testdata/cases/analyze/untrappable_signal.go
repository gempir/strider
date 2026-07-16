package analyze_cases

import (
	"os"
	"os/signal"
	"syscall"
)

func untrappableSignal(ch chan<- os.Signal) {
	signal.Notify(ch, os.Kill)
	signal.Ignore(syscall.SIGSTOP)
}
