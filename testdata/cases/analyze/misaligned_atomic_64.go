package analyze_cases

import "sync/atomic"

type misalignedCounters struct {
	ready uint32
	total uint64
}

func misalignedAtomic64(value *misalignedCounters) {
	atomic.AddUint64(&value.total, 1)
}
