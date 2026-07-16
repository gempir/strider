package analyze_cases

import "sync"

func protectedCriticalSection(mutex *sync.Mutex) {
	mutex.Lock()
	defer mutex.Unlock()
}
