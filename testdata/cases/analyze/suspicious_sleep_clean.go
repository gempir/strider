package analyze_cases

import "time"

func sleepForExplicitDuration() {
	time.Sleep(5 * time.Nanosecond)
	time.Sleep(5 * time.Second)
}
