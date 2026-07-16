package analyze_cases

import "sync"

func boxedPoolValue(pool *sync.Pool, bytes []byte) {
	pool.Put(bytes)
}
