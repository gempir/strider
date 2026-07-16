package analyze_cases

import "time"

func timerResetDrainRace(timer *time.Timer, delay time.Duration) {
	if !timer.Reset(delay) {
		<-timer.C
	}
}
