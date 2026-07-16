package analyze_cases

import (
	"os"
	"os/signal"
)

func unbufferedSignalChannel() {
	ch := make(chan os.Signal)
	signal.Notify(ch, os.Interrupt)
}
