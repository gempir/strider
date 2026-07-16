package analyze_cases

import (
	"os"
	"os/signal"
	"syscall"
)

func trappableSignal(ch chan<- os.Signal) {
	signal.Notify(ch, syscall.SIGTERM)
}
