# Phase 3 syntax-engine benchmark

Measured on 2026-07-23 on an Apple M4 with Go's `BenchmarkAnalyzeCST`,
all 97 syntax checks selected, five runs per revision:

| Revision | Time/op | Bytes/op | Allocations/op |
|---|---:|---:|---:|
| Phase 2 (`0f974d5`) | 2.24–2.27 ms | 918 kB | 16,163 |
| Phase 3 | 0.88–0.92 ms | 589 kB | 12,124 |

The independent typed callbacks and per-node fact caches reduce this focused
all-check workload by about 60% in wall time, 36% in allocated bytes, and 25%
in allocation count. `TestFunctionFactsAreBuiltOncePerDeclaration` separately
asserts that selecting all function checks derives facts once per function or
method declaration.
