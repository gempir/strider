package leakytimetickclean

import "time"

func ticker() *time.Ticker {
	return time.NewTicker(time.Second)
}
