package analyze_cases

import "sync"

func emptyCriticalSection(mutex *sync.Mutex) {
	mutex.Lock()
	mutex.Unlock()
}
