package analyze_cases

import "sync"

func deferredLockAfterLock(mutex *sync.Mutex) {
	mutex.Lock()
	defer mutex.Lock()
}
