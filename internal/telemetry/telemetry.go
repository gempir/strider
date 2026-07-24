// Package telemetry records coarse pipeline spans for benchmark and profiling
// runs. The normal path uses a nil recorder: disabled spans do not read the
// clock, allocate, or take locks.
//
//strider:ignore-file cognitive-complexity,cyclomatic-complexity,function-length,no-package-var,range-value-address,top-level-declaration-order,use-slices-sort
package telemetry

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gempir/strider/internal/buildidentity"
)

const (
	EnvironmentVariable    = "STRIDER_TELEMETRY"
	CPUProfileEnvironment  = "STRIDER_CPU_PROFILE"
	HeapProfileEnvironment = "STRIDER_HEAP_PROFILE"
)

type event struct {
	Name     string `json:"name"`
	StartNS  int64  `json:"start_ns"`
	Duration int64  `json:"duration_ns"`
}

// Phase aggregates all spans sharing a name. WallNS is the interval from the
// earliest start through the latest finish; SumNS is summed worker time.
type Phase struct {
	Name   string `json:"name"`
	Count  int    `json:"count"`
	WallNS int64  `json:"wall_ns"`
	SumNS  int64  `json:"sum_ns"`
}

type Memory struct {
	AllocBytes      uint64 `json:"alloc_bytes"`
	TotalAllocBytes uint64 `json:"total_alloc_bytes"`
	HeapAllocBytes  uint64 `json:"heap_alloc_bytes"`
	HeapObjects     uint64 `json:"heap_objects"`
	NumGC           uint32 `json:"num_gc"`
	PauseTotalNS    uint64 `json:"pause_total_ns"`
}

// MemoryPoint captures live runtime memory at a named pipeline boundary.
type MemoryPoint struct {
	Name   string `json:"name"`
	AtNS   int64  `json:"at_ns"`
	Memory Memory `json:"memory"`
}

type Report struct {
	SchemaVersion int           `json:"schema_version"`
	Command       string        `json:"command"`
	BuildIdentity string        `json:"build_identity"`
	Revision      string        `json:"revision,omitempty"`
	GoVersion     string        `json:"go_version"`
	GOOS          string        `json:"goos"`
	GOARCH        string        `json:"goarch"`
	GOMAXPROCS    int           `json:"gomaxprocs"`
	StartedAt     string        `json:"started_at"`
	DurationNS    int64         `json:"duration_ns"`
	MemoryBefore  Memory        `json:"memory_before"`
	MemoryAfter   Memory        `json:"memory_after"`
	MemoryPoints  []MemoryPoint `json:"memory_points,omitempty"`
	Phases        []Phase       `json:"phases"`
	Events        []event       `json:"events,omitempty"`
}

type recorder struct {
	path         string
	command      string
	started      time.Time
	memoryBefore runtime.MemStats
	mu           sync.Mutex
	events       []event
	memoryPoints []MemoryPoint
	cpuProfile   *os.File
}

var active atomic.Pointer[recorder]

var noopDone = func() {}

// ConfigureFromEnvironment enables one process report when STRIDER_TELEMETRY
// names an output file. Repeated calls replace the current process recorder.
func ConfigureFromEnvironment(command string) {
	path := os.Getenv(EnvironmentVariable)
	if path == "" {
		active.Store(nil)
		return
	}
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	current := &recorder{
		path:         path,
		command:      command,
		started:      time.Now(),
		memoryBefore: memory,
	}
	cpuPath := os.Getenv(CPUProfileEnvironment)
	if cpuPath != "" {
		if profile, err := createProfile(cpuPath); err == nil {
			if err := pprof.StartCPUProfile(profile); err == nil {
				current.cpuProfile = profile
			} else {
				if closeErr := profile.Close(); closeErr != nil {
					current.cpuProfile = nil
				}
			}
		}
	}
	active.Store(current)
}

// Start begins a coarse span. When telemetry is disabled it returns a shared
// no-op closure without reading the clock or allocating.
func Start(name string) func() {
	current := active.Load()
	if current == nil {
		return noopDone
	}
	started := time.Now()
	return func() {
		finished := time.Now()
		current.mu.Lock()
		current.events = append(current.events, event{
			Name:     name,
			StartNS:  started.Sub(current.started).Nanoseconds(),
			Duration: finished.Sub(started).Nanoseconds(),
		})
		current.mu.Unlock()
	}
}

// Snapshot captures live memory at a coarse pipeline boundary. Disabled
// telemetry returns before reading runtime memory.
func Snapshot(name string) {
	current := active.Load()
	if current == nil {
		return
	}
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	current.mu.Lock()
	current.memoryPoints = append(current.memoryPoints, MemoryPoint{
		Name:   name,
		AtNS:   time.Since(current.started).Nanoseconds(),
		Memory: memorySnapshot(&memory),
	})
	current.mu.Unlock()
}

// Flush atomically writes the active report. A missing recorder is a no-op.
func Flush() error {
	current := active.Swap(nil)
	if current == nil {
		return nil
	}
	if current.cpuProfile != nil {
		pprof.StopCPUProfile()
		if err := current.cpuProfile.Close(); err != nil {
			return err
		}
	}
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	current.mu.Lock()
	events := append([]event(nil), current.events...)
	memoryPoints := append([]MemoryPoint(nil), current.memoryPoints...)
	current.mu.Unlock()
	sort.SliceStable(
		events,
		func(left, right int) bool {
			if events[left].StartNS != events[right].StartNS {
				return events[left].StartNS < events[right].StartNS
			}
			return events[left].Name < events[right].Name
		},
	)
	report := Report{
		SchemaVersion: 1,
		Command:       current.command,
		BuildIdentity: buildidentity.Identity(),
		Revision:      buildidentity.Revision(),
		GoVersion:     runtime.Version(),
		GOOS:          runtime.GOOS,
		GOARCH:        runtime.GOARCH,
		GOMAXPROCS:    runtime.GOMAXPROCS(0),
		StartedAt:     current.started.UTC().Format(time.RFC3339Nano),
		DurationNS:    time.Since(current.started).Nanoseconds(),
		MemoryBefore:  memorySnapshot(&current.memoryBefore),
		MemoryAfter:   memorySnapshot(&after),
		MemoryPoints:  memoryPoints,
		Phases:        aggregate(events),
		Events:        events,
	}
	contents, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	contents = append(contents, '\n')
	if err := os.MkdirAll(filepath.Dir(current.path), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(current.path), ".strider-telemetry-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	cleanup := func(cause error) error {
		return errors.Join(cause, temporary.Close(), os.Remove(temporaryPath))
	}
	if _, err := temporary.Write(contents); err != nil {
		return cleanup(err)
	}
	if err := temporary.Sync(); err != nil {
		return cleanup(err)
	}
	if err := temporary.Close(); err != nil {
		return errors.Join(err, os.Remove(temporaryPath))
	}
	if err := os.Rename(temporaryPath, current.path); err != nil {
		return errors.Join(err, os.Remove(temporaryPath))
	}
	heapPath := os.Getenv(HeapProfileEnvironment)
	if heapPath != "" {
		profile, err := createProfile(heapPath)
		if err != nil {
			return err
		}
		if err := pprof.WriteHeapProfile(profile); err != nil {
			return errors.Join(err, profile.Close())
		}
		if err := profile.Close(); err != nil {
			return err
		}
	}
	return nil
}

func createProfile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return os.Create(path)
}

func memorySnapshot(stats *runtime.MemStats) Memory {
	return Memory{
		AllocBytes:      stats.Alloc,
		TotalAllocBytes: stats.TotalAlloc,
		HeapAllocBytes:  stats.HeapAlloc,
		HeapObjects:     stats.HeapObjects,
		NumGC:           stats.NumGC,
		PauseTotalNS:    stats.PauseTotalNs,
	}
}

func aggregate(events []event) []Phase {
	type bounds struct {
		phase  Phase
		start  int64
		finish int64
	}
	byName := make(map[string]*bounds)
	for _, item := range events {
		current := byName[item.Name]
		if current == nil {
			current = &bounds{
				phase: Phase{
					Name: item.Name,
				},
				start:  item.StartNS,
				finish: item.StartNS + item.Duration,
			}
			byName[item.Name] = current
		}
		current.phase.Count++
		current.phase.SumNS += item.Duration
		current.start = min(current.start, item.StartNS)
		current.finish = max(current.finish, item.StartNS+item.Duration)
	}
	result := make([]Phase, 0, len(byName))
	for _, item := range byName {
		item.phase.WallNS = item.finish - item.start
		result = append(result, item.phase)
	}
	sort.Slice(result, func(left, right int) bool {
		return result[left].Name < result[right].Name
	})
	return result
}
