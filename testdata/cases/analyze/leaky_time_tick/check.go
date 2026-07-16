package leakytimetick

import "time"

func ticks() <-chan time.Time {
	return time.Tick(time.Second)
}
