package analyze_cases

import "time"

func resetTimer(timer *time.Timer, delay time.Duration) {
	timer.Reset(delay)
}
