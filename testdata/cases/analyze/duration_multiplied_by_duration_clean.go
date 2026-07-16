package analyze_cases

import "time"

func scaledDuration(count int) time.Duration {
	return time.Duration(count) * time.Second
}
