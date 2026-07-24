// Package fileschedule provides bounded worker scheduling strategies for
// independent file-local work.
package fileschedule

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"slices"
	"sync"
)

const (
	EnvironmentVariable          = "STRIDER_FILE_SCHEDULER"
	FIFO                Strategy = "fifo"
	LargestFirst        Strategy = "largest-first"
	WorkStealing        Strategy = "work-stealing"
)

type Strategy string

// Resolve reads and validates the process scheduling strategy.
func Resolve() (Strategy, error) {
	value := Strategy(os.Getenv(EnvironmentVariable))
	if value == "" {
		return WorkStealing, nil
	}
	switch value {
	case FIFO, LargestFirst, WorkStealing:
		return value, nil
	default:
		return "", fmt.Errorf("unsupported file scheduler %q", value)
	}
}

// Order returns an independent item order. Largest-first calls size once per
// item and preserves input order for ties.
func Order[T any](items []T, strategy Strategy, size func(T) (int64, error)) ([]T, error) {
	type weighted struct {
		item T
		size int64
	}
	if strategy != LargestFirst {
		return append([]T(nil), items...), nil
	}
	weightedItems := make([]weighted, 0, len(items))
	for _, item := range items {
		itemSize, err := size(item)
		if err != nil {
			return nil, err
		}
		weightedItems = append(weightedItems, weighted{
			item: item,
			size: itemSize,
		})
	}
	slices.SortStableFunc(weightedItems, func(left, right weighted) int {
		return cmp.Compare(right.size, left.size)
	})
	result := make([]T, 0, len(items))
	for _, item := range weightedItems {
		result = append(result, item.item)
	}
	return result, nil
}

// Run processes every item with static FIFO assignment or a shared dynamic
// queue. Largest-first uses the shared queue after Order.
func Run[T any](ctx context.Context, strategy Strategy, workers int, items []T, run func(context.Context, T)) {
	if len(items) == 0 || workers <= 0 || run == nil {
		return
	}
	if strategy == FIFO {
		runStatic(ctx, workers, items, run)
		return
	}
	runDynamic(ctx, workers, items, run)
}

func runStatic[T any](ctx context.Context, workers int, items []T, run func(context.Context, T)) {
	var group sync.WaitGroup
	for worker := range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for index := worker; index < len(items); index += workers {
				if ctx.Err() != nil {
					return
				}
				run(ctx, items[index])
			}
		}()
	}
	group.Wait()
}

func runDynamic[T any](ctx context.Context, workers int, items []T, run func(context.Context, T)) {
	jobs := make(chan T)
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			runWorker(ctx, jobs, run)
		}()
	}
dispatch:
	for _, item := range items {
		select {
		case <-ctx.Done():
			break dispatch
		case jobs <- item:
		}
	}
	close(jobs)
	group.Wait()
}

func runWorker[T any](ctx context.Context, jobs <-chan T, run func(context.Context, T)) {
	for {
		select {
		case <-ctx.Done():
			return
		case item, ok := <-jobs:
			if !ok {
				return
			}
			run(ctx, item)
		}
	}
}
