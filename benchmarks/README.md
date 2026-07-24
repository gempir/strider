# Performance benchmark protocol

The corpus runner implements the protocol in `PLAN.md`. A routine run uses one
warm-up and seven measured samples for every category:

```sh
make corpus-check
```

Use at least 20 samples before publishing p95 values:

```sh
make corpus-check CORPUS_FLAGS="--samples 25"
```

The fixed `GOMAXPROCS=2` result is the comparison gate. Native-core results are
informational. Package-aware checks report cold and warm Go build caches
separately; the module download cache remains populated and is recorded in the
report. Cold state is recreated for every measured process. Warm state receives
an exact-binary/configuration population run before measurement. OS filesystem
cache state is explicitly accepted as warm.

The runner also measures `check --no-package-loading` as `check-file-local`.
That lane isolates persistent-cache lookup and diagnostic materialization from
the package-loading, type-analysis, and SSA floor in a full check.

`target/corpus/report.json` contains the environment, all raw samples, medians,
p95, allocations, GC cycles and pause time, external peak RSS, and aggregate
phase spans. Detailed event traces live below `target/corpus/raw`. Aggregate
parallel phases distinguish critical-path wall time from summed worker time.
Set `STRIDER_CPU_PROFILE` or `STRIDER_HEAP_PROFILE` alongside
`STRIDER_TELEMETRY` when a focused run needs Go pprof output.

Scheduling experiments use `STRIDER_FILE_SCHEDULER=fifo`,
`largest-first`, or `work-stealing`. Dynamic work stealing is the default.
`STRIDER_OVERLAP_PACKAGE_LOADING=1` is an evaluation-only switch; package
loading remains sequential by default because the Phase 6 fixed- and
native-core runs both regressed with overlap.

For a focused SFTPGo comparison:

```sh
make corpus-check CORPUS_FLAGS="--project sftpgo"
```

`benchmarks/performance-baseline.json` is the Phase 0 fixed-core SFTPGo
re-baseline captured before the optimization phases. Digest changes are
correctness failures and must not be refreshed as performance baselines.
