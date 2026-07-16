package analyze_cases

import "sync"

func deferredUnlockAfterLock(mutex *sync.Mutex) {
	mutex.Lock()
	defer mutex.Unlock()
}
