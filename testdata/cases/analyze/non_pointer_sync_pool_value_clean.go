package analyze_cases

import "sync"

func pointerPoolValue(pool *sync.Pool, bytes []byte) {
	pool.Put(&bytes)
}
