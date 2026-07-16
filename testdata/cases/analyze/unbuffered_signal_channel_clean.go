package analyze_cases

import (
	"os"
	"os/signal"
)

func bufferedSignalChannel() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
}
