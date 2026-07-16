package analyze_cases

import "sync"

func waitGroupAddInsideGoroutine(group *sync.WaitGroup) {
	go func() {
		group.Add(1)
		defer group.Done()
	}()
}
