package workspace

import (
	"context"
	"errors"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCSTAdmissionBlocksUntilBytesAreReleased(t *testing.T) {
	admission := NewCSTAdmission(10)
	releaseFirst, err := admission.Acquire(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	acquired := make(chan func(), 1)
	go func() {
		release, acquireErr := admission.Acquire(context.Background(), 7)
		if acquireErr == nil {
			acquired <- release
		}
	}()
	select {
	case <-acquired:
		t.Fatal("second reservation acquired before capacity was released")
	case <-time.After(10 * time.Millisecond):
	}
	releaseFirst()
	select {
	case releaseSecond := <-acquired:
		releaseSecond()
	case <-time.After(time.Second):
		t.Fatal("second reservation did not acquire released capacity")
	}
}

func TestCSTAdmissionCancellation(t *testing.T) {
	admission := NewCSTAdmission(10)
	release, err := admission.Acquire(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = admission.Acquire(ctx, 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("acquire error = %v, want context cancellation", err)
	}
}

func TestCSTAdmissionOversizeEstimateGetsExclusiveCapacity(t *testing.T) {
	admission := NewCSTAdmission(10)
	release, err := admission.Acquire(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	release()
	release()
}

func TestCSTAdmissionBoundsSeveralLargeConcurrentFiles(t *testing.T) {
	admission := NewCSTAdmission(100)
	start := make(chan struct{})
	var live atomic.Int64
	var maximum atomic.Int64
	var group sync.WaitGroup
	for range 8 {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			release, err := admission.Acquire(context.Background(), 60)
			if err != nil {
				t.Error(err)
				return
			}
			current := live.Add(60)
			storeMaximum(&maximum, current)
			live.Add(-60)
			release()
		}()
	}
	close(start)
	group.Wait()
	if got := maximum.Load(); got > 100 {
		t.Fatalf("maximum admitted estimate = %d, capacity 100", got)
	}
}

func storeMaximum(maximum *atomic.Int64, value int64) {
	for {
		previous := maximum.Load()
		if value <= previous || maximum.CompareAndSwap(previous, value) {
			return
		}
	}
}

func TestEstimatedCSTBytesUsesFloorAndSourceMultiplier(t *testing.T) {
	if got := EstimatedCSTBytes(1); got != cstEstimateFloor {
		t.Fatalf("small estimate = %d, want floor %d", got, cstEstimateFloor)
	}
	var sourceBytes int64 = cstEstimateFloor
	if got := EstimatedCSTBytes(sourceBytes); got != sourceBytes*cstEstimateMultiplier {
		t.Fatalf("large estimate = %d", got)
	}
}

func TestCSTCollectionWindowRespectsExplicitGOGC(t *testing.T) {
	t.Setenv("GOGC", "off")
	previous := debug.SetGCPercent(77)
	t.Cleanup(func() {
		debug.SetGCPercent(previous)
	})
	restore := BeginCSTCollectionWindow()
	if got := debug.SetGCPercent(77); got != 77 {
		t.Fatalf("GC percent changed to %d despite explicit GOGC", got)
	}
	restore()
}

func TestCSTCollectionWindowRestoresDefault(t *testing.T) {
	t.Setenv("GOGC", "")
	previous := debug.SetGCPercent(77)
	t.Cleanup(func() {
		debug.SetGCPercent(previous)
	})
	restoreFirst := BeginCSTCollectionWindow()
	restoreSecond := BeginCSTCollectionWindow()
	if got := debug.SetGCPercent(cstGCPercent); got != cstGCPercent {
		t.Fatalf("GC percent in nested window = %d", got)
	}
	restoreFirst()
	if got := debug.SetGCPercent(cstGCPercent); got != cstGCPercent {
		t.Fatalf("GC percent restored before final window = %d", got)
	}
	restoreSecond()
	if got := debug.SetGCPercent(77); got != 77 {
		t.Fatalf("restored GC percent = %d, want 77", got)
	}
}
