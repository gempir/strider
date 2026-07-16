package analyze_cases

import "sync"

func waitGroupAddBeforeGoroutine(group *sync.WaitGroup) {
	group.Add(1)
	go func() {
		defer group.Done()
	}()
}
