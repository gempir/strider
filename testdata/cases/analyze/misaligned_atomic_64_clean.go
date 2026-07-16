package analyze_cases

import "sync/atomic"

type alignedCounters struct {
	total uint64
	ready uint32
}

func alignedAtomic64(value *alignedCounters) {
	atomic.AddUint64(&value.total, 1)
}
