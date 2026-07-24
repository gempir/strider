package telemetry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDisabledStartIsSharedNoop(t *testing.T) {
	active.Store(nil)
	first := Start("one")
	second := Start("two")
	Snapshot("memory")
	first()
	second()
	if active.Load() != nil {
		t.Fatal("disabled span installed a recorder")
	}
}

func TestFlushAggregatesParallelWallAndSummedTime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	t.Setenv(EnvironmentVariable, path)
	ConfigureFromEnvironment("check")
	current := active.Load()
	current.events = []event{
		{
			Name:     "worker",
			StartNS:  10,
			Duration: 30,
		},
		{
			Name:     "worker",
			StartNS:  20,
			Duration: 50,
		},
	}
	current.started = time.Now()
	Snapshot("after-workers")
	if err := Flush(); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var report Report
	if err := json.Unmarshal(contents, &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Phases) != 1 {
		t.Fatalf("phases = %+v", report.Phases)
	}
	if report.Phases[0].WallNS != 60 || report.Phases[0].SumNS != 80 || report.Phases[0].Count != 2 {
		t.Fatalf("worker aggregate = %+v", report.Phases[0])
	}
	if len(report.MemoryPoints) != 1 || report.MemoryPoints[0].Name != "after-workers" {
		t.Fatalf("memory points = %+v", report.MemoryPoints)
	}
}
