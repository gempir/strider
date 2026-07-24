//strider:ignore-file no-package-var
package workspace

import (
	"context"
	"errors"
	"os"
	"runtime/debug"
	"sync"
)

const (
	defaultCSTAdmissionBytes = 256 << 20
	cstGCPercent             = 300
)

var cstGCTuning = struct {
	sync.Mutex
	users    int
	previous int
}{}

// CSTAdmission bounds the estimated live CST heap across concurrent file
// workers. Estimates larger than the capacity receive exclusive admission.
type CSTAdmission struct {
	mu       sync.Mutex
	capacity int64
	used     int64
	changed  chan struct{}
}

// NewCSTAdmission constructs a byte-weighted CST admission gate.
// Non-positive capacities select the conservative default.
func NewCSTAdmission(capacity int64) *CSTAdmission {
	resolvedCapacity := capacity
	if resolvedCapacity <= 0 {
		resolvedCapacity = defaultCSTAdmissionBytes
	}
	return &CSTAdmission{
		capacity: resolvedCapacity,
		changed:  make(chan struct{}),
	}
}

// BeginCSTCollectionWindow raises the default GC target while the admission
// gate and per-file release bound the live CST set. An explicit GOGC value
// always wins. Overlapping in-process operations share one tuning window.
func BeginCSTCollectionWindow() func() {
	if os.Getenv("GOGC") != "" {
		return func() {}
	}
	cstGCTuning.Lock()
	if cstGCTuning.users == 0 {
		cstGCTuning.previous = debug.SetGCPercent(cstGCPercent)
	}
	cstGCTuning.users++
	cstGCTuning.Unlock()
	var once sync.Once
	return func() {
		once.Do(endCSTCollectionWindow)
	}
}

func endCSTCollectionWindow() {
	cstGCTuning.Lock()
	cstGCTuning.users--
	if cstGCTuning.users == 0 {
		debug.SetGCPercent(cstGCTuning.previous)
	}
	cstGCTuning.Unlock()
}

// Acquire reserves estimated CST bytes until the returned release function is
// called. The release function is safe to call repeatedly.
func (admission *CSTAdmission) Acquire(ctx context.Context, estimatedBytes int64) (func(), error) {
	if admission == nil {
		return func() {}, nil
	}
	if ctx == nil {
		return nil, errors.New("acquire CST admission: nil context")
	}
	weight := admission.weight(estimatedBytes)
	for {
		changed, release, acquired := admission.tryAcquire(weight)
		if acquired {
			return release, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-changed:
		}
	}
}

func (admission *CSTAdmission) weight(estimatedBytes int64) int64 {
	if estimatedBytes <= 0 {
		return min(cstEstimateFloor, admission.capacity)
	}
	return min(estimatedBytes, admission.capacity)
}

func (admission *CSTAdmission) tryAcquire(weight int64) (chan struct{}, func(), bool) {
	admission.mu.Lock()
	defer admission.mu.Unlock()
	if admission.used+weight > admission.capacity {
		return admission.changed, nil, false
	}
	admission.used += weight
	var once sync.Once
	return nil, func() {
		once.Do(func() {
			admission.release(weight)
		})
	}, true
}

func (admission *CSTAdmission) release(weight int64) {
	admission.mu.Lock()
	admission.used -= weight
	close(admission.changed)
	admission.changed = make(chan struct{})
	admission.mu.Unlock()
}
