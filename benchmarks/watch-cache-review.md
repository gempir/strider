# Watch cache review

This review decides whether watch mode should keep both the bounded workspace
snapshot/CST cache and the whole-concrete-result `checks.Session` cache.

## Acceptance gate

The second cache earns its 233-line fingerprinting, cloning, and lifecycle
surface only if it reduces the median wall time of an unchanged large
generation by at least 20%, or avoids package-aware work. It must also avoid a
greater than 5% regression when one source file changes. Allocation savings
alone are insufficient because watch latency is user-visible and the workspace
cache already owns the large source and CST objects.

## Method

The benchmark used a two-file module and an 80-file module. Each selected one
concrete check (`no-init`) and one type check (`copy-lock-value`) so the
package-load work that dominates normal watch iterations remained present.
Dependency files were excluded from the concrete workspace snapshot but loaded
by `go/packages`, matching a watched package whose imported package changes.

Command and environment:

```text
GOMAXPROCS=2 go test ./internal/checks -run '^$' \
  -bench '^BenchmarkWatchCacheMatrix$' -benchtime=5x -count=3
darwin/arm64, Apple M4
```

Values below are medians of three runs. Bytes and allocations are per
iteration. The A/B harness was temporary because it depended on the rejected
`checks.Session`; the retained workspace cache continues to have a permanent
`BenchmarkCacheUnchangedGeneration` benchmark.

| Size | Scenario | Cache | Time | Bytes | Allocs | Workspace hit/miss | Result hit/miss |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: |
| small | unchanged | workspace | 17.47 ms | 164,892 | 1,441 | 2.0 / 0 | — |
| small | unchanged | workspace + result | 17.02 ms | 149,257 | 1,263 | 2.0 / 0 | 1 / 0 |
| small | source changed | workspace | 20.04 ms | 170,220 | 1,500 | 1.6 / 0.4 | — |
| small | source changed | workspace + result | 20.24 ms | 170,798 | 1,505 | 1.6 / 0.4 | 0 / 1 |
| small | dependency changed | workspace | 33.04 ms | 821,560 | 6,898 | 2.0 / 0 | — |
| small | dependency changed | workspace + result | 33.10 ms | 804,851 | 6,720 | 2.0 / 0 | 1 / 0 |
| large | unchanged | workspace | 26.41 ms | 1,804,452 | 16,453 | 80 / 0 | — |
| large | unchanged | workspace + result | 25.17 ms | 1,169,316 | 9,876 | 80 / 0 | 1 / 0 |
| large | source changed | workspace | 30.87 ms | 1,815,924 | 16,511 | 79.6 / 0.4 | — |
| large | source changed | workspace + result | 30.53 ms | 1,841,876 | 16,673 | 79.6 / 0.4 | 0 / 1 |
| large | dependency changed | workspace | 46.18 ms | 3,318,267 | 27,705 | 80 / 0 | — |
| large | dependency changed | workspace + result | 43.90 ms | 2,666,305 | 21,130 | 80 / 0 | 1 / 0 |

A separate five-iteration retained-heap sample measured about 10 KiB retained
after warming the small fixture and 309–315 KiB for the large fixture in both
modes. The result cache did not create a distinguishable retained-heap benefit
or cost beyond measurement noise; the workspace cache owns the retained source
and CST data.

## Decision

Delete `checks.Session`. Its best relevant median wall-time improvement was
4.9%, far below the 20% gate, and semantic package loading still ran on every
iteration. It reduced allocations for unchanged and dependency-only large
iterations, but every source edit invalidated the whole concrete result and
added fingerprinting plus clone work.

Watch mode retains the bounded workspace snapshot/CST cache. Each generation is
closed by its caller after a direct `checks.Run`, and output is suppressed by
comparing the owned diagnostics from the previous completed generation. This
leaves one cache with one responsibility and no duplicated module-boundary
fingerprint.
