package fileschedule

import (
	"context"
	"reflect"
	"sort"
	"sync"
	"testing"
)

func TestResolve(t *testing.T) {
	t.Setenv(EnvironmentVariable, "")
	if got, err := Resolve(); err != nil || got != WorkStealing {
		t.Fatalf("default strategy = %q, %v", got, err)
	}
	t.Setenv(EnvironmentVariable, "largest-first")
	if got, err := Resolve(); err != nil || got != LargestFirst {
		t.Fatalf("explicit strategy = %q, %v", got, err)
	}
	t.Setenv(EnvironmentVariable, "unknown")
	if _, err := Resolve(); err == nil {
		t.Fatal("invalid strategy was accepted")
	}
}

func TestLargestFirstOrderIsStable(t *testing.T) {
	items := []string{
		"small-a",
		"large-a",
		"small-b",
		"large-b",
	}
	sizes := map[string]int64{
		"small-a": 1,
		"large-a": 2,
		"small-b": 1,
		"large-b": 2,
	}
	got, err := Order(items, LargestFirst, func(item string) (int64, error) {
		return sizes[item], nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"large-a",
		"large-b",
		"small-a",
		"small-b",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestStrategiesRunEveryItem(t *testing.T) {
	for _, strategy := range []Strategy{
		FIFO,
		LargestFirst,
		WorkStealing,
	} {
		t.Run(
			string(strategy),
			func(t *testing.T) {
				var mu sync.Mutex
				visited := []int{}
				Run(
					context.Background(),
					strategy,
					4,
					[]int{
						0,
						1,
						2,
						3,
						4,
						5,
					},
					func(_ context.Context, item int) {
						mu.Lock()
						visited = append(visited, item)
						mu.Unlock()
					},
				)
				sort.Ints(visited)
				if !reflect.DeepEqual(visited, []int{
					0,
					1,
					2,
					3,
					4,
					5,
				}) {
					t.Fatalf("visited = %v", visited)
				}
			},
		)
	}
}
