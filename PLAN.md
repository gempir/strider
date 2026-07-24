# Strider Performance Plan

Status: active
Primary comparison project: SFTPGo
Scope: `strider fmt --check` and `strider check`

## Objective

Reduce Strider's formatting and checking latency on large Go repositories
without weakening formatter correctness, diagnostic coverage, determinism, or
benchmark comparability.

Historical context (single timed run, not an acceptance baseline):

| Operation | Published (GOMAXPROCS=2, single run) |
| --- | ---: |
| Format | 1,362 ms |
| Check | 3,877 ms |

All acceptance gates compare against a fresh Phase 0 baseline measured under
the final protocol, on the same machine and Go toolchain. Absolute-millisecond
targets are provisional planning estimates only; gates are expressed as
relative improvements over that baseline. The fixed two-core corpus result
remains the primary historical comparison; native-core results are
informational and must never substitute for it.

## Key findings

Verified against the current code:

- The read-only format path performs full convergence work. An
  already-formatted file costs at minimum two renders, two `go/format.Source`
  calls, and one full reparse of the formatted output just to confirm
  stability (`internal/formatter/formatter.go:125-171`). Profiling attributed
  roughly half of formatter CPU to this preview/convergence path.
- `fmt --check` retains the complete formatted source for every file
  (`internal/app/format_files.go:90-94`) even though only `Changed` is read.
  `strider check` already drops candidates unless fix mode sets
  `CollectCandidates` (`internal/checks/run.go`, `internal/app/check.go:211`)
  — that half of the retention problem is done.
- The workspace retains all parsed CSTs until the operation completes.
  `File.releaseCST()` already exists (`internal/workspace/workspace.go:222`)
  but is only invoked from `Workspace.Close()`. Observed on SFTPGo:
  ~32 GC cycles, ~20% of runtime in GC, peak heap ~367 MB.
- A bounded, byte-pressure-aware CST cache with a tree-size estimate already
  exists for watch mode (`internal/workspace/cache.go`); reuse its size
  estimate rather than inventing a second one.
- The syntax engine rebuilds the node-kind dispatch map for every file
  (`internal/checks/syntax/engine.go:49`) and recomputes `activeChecks` per
  file (`internal/checks/syntax/syntax.go:76-85`). `cst.NodeTokens`, range,
  and token-bound helpers walk subtrees repeatedly and allocate token slices.
  `Pass.functions` is keyed by `cst.Node` interface values.
- `source.DiagnosticPath` performs absolute-path and symlink resolution per
  call.
- Formatter output depends on the module path resolved from the nearest
  `go.mod` (`internal/formatter/formatter.go:174`,
  `internal/formatter/module_cache.go`), and native syntax checks inspect
  path-derived facts (filename format, `_test.go`, `testdata`, directory
  name). Both affect cache-key design.
- There is no persistent cache between CLI invocations.
- A cold Go build cache dominates cold checks (~32 s vs ~3.6 s warm). This is
  a separate performance mode and must be measured separately.

## Benchmark protocol

### Workloads and environment

Use the pinned SFTPGo revision from `benchmarks/projects.json` and the
existing corpus commands:

```text
strider --no-config fmt --check .
strider --no-config check --minimum-severity note --format json .
```

Preserve the corpus environment (`CGO_ENABLED=0`, `GOARCH=amd64`,
`GOOS=linux`, `GOWORK=off`). Note when publishing: runs on the development
machine are cross-compilation numbers for package loading and Go-build-cache
phases and may differ from native-user latency.

Run every benchmark in both scheduler modes (`GOMAXPROCS=2` as the historical
gate; native cores as informational) and both Strider cache modes (cold,
warm). Report cold and warm Go build cache separately for package-aware
checks.

### Cache state definitions

- Cold Strider cache: fresh, empty cache directory for every measured process.
- Warm Strider cache: preceded by a successful population run using the exact
  same binary and configuration.
- Cold Go build cache: fresh `GOCACHE` per measured process; `GOMODCACHE`
  remains populated and is reported separately.
- A cold sample must not have a warm-up that silently warms the state being
  measured. State whether the OS filesystem cache is controlled or accepted
  as warm.

### Statistics

- Seven measured samples (median only) for routine PR checks; at least 20–30
  samples when publishing p95.
- Store all raw samples; use a `benchstat`-style comparison; interleave
  before/after runs where practical.
- Record Go version, Strider revision/build identity, CPU model, core count,
  memory, OS, and corpus checkout revision with every result.
- For parallel phases, distinguish wall-clock span on the critical path from
  summed per-file worker time; they are not interchangeable.
- Peak RSS comes from an external process measurement, never a sampled Go
  heap value.
- Separate raw `cst.Parse` time from render, `go/format`, and traversal time
  so the Phase 5 go/no-go decision has the number it needs.
- Detailed per-file statistics (per-file p50/p95, bytes retained per source
  byte, index sizes) are collected when the phase that needs them starts, not
  all up front.
- A single best run is never the published result. Do not compare the
  combined `strider check` number with file-local untyped linters without
  publishing separated phase results.

### Instrumentation

Use a nil/no-op recorder so the normal path performs no per-node or
per-diagnostic timing calls. Acceptance for disabled instrumentation is "no
statistically significant difference at the required sample count", which is
testable, rather than an unverifiable <1% bound. A profiling mode may cost
more. Add end-to-end and allocation benchmarks for representative small,
medium, and ~1 MB Go files; the existing fixtures are too small to catch the
retention behavior motivating this plan.

## Phases

### Phase 0: Measurement and statistically valid re-baseline

- [ ] Add structured phase timing for the formatter and check pipeline with
  the parallel-phase semantics above.
- [ ] Add allocation, GC, and external peak-RSS collection to benchmark runs.
- [ ] Change corpus timing from a single run to warm-up plus repeated samples
  with raw-sample storage.
- [ ] Add fixed-core/native-core and cold/warm result categories; store the
  benchmark environment and Strider revision in reports.
- [ ] Re-baseline SFTPGo under the final protocol; all later gates compare to
  this baseline.
- [ ] Establish the warm semantic floor: measure `go/packages`, type
  analysis, SSA, sorting, and reporting so later warm full-check targets are
  bounded by reality.
- [ ] Attribute time across parse, render, `go/format`, and traversal.
- [ ] Run the one-line `GOGC`/`GOMEMLIMIT` experiment to size how much of the
  ~20% GC cost is avoidable without structural change (measurement input
  only, not a substitute for the lifetime fix).

Acceptance:

- Existing diagnostic digests unchanged.
- Variance low enough to detect a 5% regression.
- Disabled instrumentation statistically indistinguishable from the previous
  binary.

### Phase 1: Status-only formatting fast path and cheap wins

Introduce:

```go
func (f *Formatter) WouldChangeTree(
    filename string,
    tree *cst.Tree,
    options Options,
) (changed bool, ignored bool, err error)
```

Implementation: check the ignore directive, render the original CST once, run
`go/format` once, byte-compare with the original, return status only.

Contract (explicit):

- `WouldChangeTree(source) == FormatTree(source).Changed` for every input on
  which the full formatter succeeds; a differential test across the entire
  corpus enforces this, and any `Changed`/`Ignored` disagreement is a
  fast-path bug, not an acceptable approximation.
- Convergence-error parity is intentionally not guaranteed: the fast path
  guarantees drift status, parse/render errors, and ignore behavior only.
- The API takes no context; cancellation is handled by caller-side context
  checks immediately before and after the formatter call.

Also in this phase:

- [ ] Early exit inside `previewTree`: if the first render+format equals the
  source, skip the confirmation reparse/render. Benefits every caller,
  including write mode on already-formatted files.
- [ ] Stop retaining source in `fmt --check`: replace the `formattedFile`
  retention with a lightweight status-only result.
- [ ] Exactly one layer owns the format-ignore decision per path.
- [ ] Cheap wins pulled forward: build the node-kind dispatch map once per
  registry/configuration (accounting for per-file exclusions), compute
  `activeChecks` once, and materialize diagnostic display paths once per run.

Lifecycle matrix (governs this phase and Phase 3):

| Mode | Retain original source | Retain candidate | Retain CST after file work |
| --- | --- | --- | --- |
| `fmt --check` | No | No | No |
| read-only `check` | No | No | No |
| `fmt --diff` | Yes | Yes | No |
| `fmt --write` | Until batch write | Until batch write | No |
| check fix mode | As required by fix planning | Only when required | No |
| watch mode | Per bounded cache policy | No by default | Yes, while cached |

Use the fast path when `fmt --check` needs no diff and when `strider check`
runs the format diagnostic with `CollectCandidates == false`. The full
convergence/verification path remains for writes, diffs, fix application, and
any API returning a candidate.

Tests: already-formatted files, files requiring formatting, ignored files,
import grouping, trivia movement, inputs requiring multiple convergence
renders, identical write/diff results, fuzzing the boolean path against the
full path. Note the fast path still resolves the module path and scans tokens
for imports; include that in its profile.

Acceptance (vs Phase 0 baseline, two cores):

- ≥35% lower median SFTPGo format-check time.
- ≥15% lower median SFTPGo full-check time.
- No finding or output digest changes.

### Phase 2: Persistent file-local result cache

Promoted ahead of lifetime work: warm runs dominate real usage and this
depends only on Phases 0–1.

Cache only file-local results: formatting drift status and native syntax
diagnostics.

Cache key must include:

- Source-content hash.
- Strider build/revision identity suitable for development binaries (not only
  a release version string) and cache schema version.
- Formatter configuration; enabled syntax checks, severities, resolved rule
  options; ignore and exclusion configuration.
- Normalized root-relative logical path or a digest of all path-derived rule
  inputs (filename format, `_test.go`, `testdata`, directory name).
- Effective module path and the identity of the `go.mod` used for import
  categorization.
- Target platform and file-selection inputs where they affect discovery.

Entry model: path-neutral diagnostics (code, message, byte offsets, complete
fix data). Materialize display paths, positions, effective severity,
exclusions, and final sorting per invocation. Never persist absolute paths.

Required behavior: atomic writes, concurrent-process safety, corrupt entries
become safe misses, bounded size with predictable eviction, explicit
disable/clear. File metadata (size/mtime) may fast-reject a candidate only;
content identity is the sole positive-hit boundary.

Benchmark the whole hit path: discovery, reading/hashing, lookup, diagnostic
materialization, sorting, and reporting — fixed costs remain even when
analysis is free.

Targets are mode-specific, derived from Phase 0 floors:

- A warm unchanged `fmt --check` target.
- A warm file-local check lane target (e.g. `check --no-package-loading`).
- A warm full-check target equal to the measured semantic/reporting floor
  plus a bounded lookup allowance. Do not gate this phase on uncached
  semantic phases it cannot control.

Acceptance:

- Cache hit and miss results have identical diagnostic digests.
- Corruption and schema-upgrade tests pass.
- Mode-specific warm targets met.

### Phase 3: CST lifetime and one-shot per-file release

Extend the existing `releaseCST`/`release` mechanics rather than adding a
second lifecycle system:

- [ ] Release a file's CST after its last consumer (formatter plus native
  syntax checks) in one-shot commands; decide whether release is
  runner-owned or exposed as a one-shot `Release` method.
- [ ] For read-only one-shot commands, release the complete per-file state
  once diagnostics are materialized; diff/write/fix modes release the CST but
  retain exactly the byte slices their transactional step needs (per the
  lifecycle matrix).
- [ ] Cached watch snapshots must not be invalidated by generation-local
  release; add race tests for release concurrent with reads and explicit
  watch-reuse tests.
- [ ] Byte-weighted concurrency/admission limit reusing the watch-cache tree
  size estimate so several very large files cannot create unbounded live
  memory.
- [ ] Capacity estimates from source size where measurements show a stable
  relationship.
- [ ] Replace short-lived per-node buffers with a small number of
  operation-owned scratch buffers (checkpoint/reset). Scratch buffers are
  pools by another name: the same bounded-lifetime requirement applies, and
  no pool is introduced without a proven bounded lifetime.

Acceptance:

- ≥30% lower peak heap on SFTPGo and ≥30% fewer GC cycles or GC CPU,
  evidenced by allocated bytes, live heap after the concrete phase, and
  external peak RSS — GC-cycle count alone is insufficient.
- No watch-mode regression; no change to diagnostic ordering or determinism.

### Phase 4: Measured query improvements, then re-profile

- [ ] Non-allocating token iterator or callback API; migrate hot syntax
  checks off temporary `[]Token` allocations.
- [ ] Store or precompute start/end offsets for production nodes; replace
  repeated subtree walks in range and token-bound helpers.
- [ ] Re-profile end to end and re-rank the remaining work. This checkpoint
  decides whether Phase 5 happens at all.

Acceptance: ≥10% lower native-syntax CPU on SFTPGo; no digest changes.

### Phase 5 (optional, go/no-go): Compact sidecar index

Begin only if the Phase 4 profile shows CST queries still on the critical
path. Instrument call counts and time first; store only fields used by proven
hot consumers rather than a full second tree.

- Dense `uint32` node/token IDs from one source-order traversal; kind, start,
  and end offsets in dense arrays.
- Fixed children as IDs; variable children as `(start, length)` ranges into
  one shared array; token spellings as source offsets where possible.
- Replace hot `map[cst.Node]` lookups (e.g. `Pass.functions`) with indexed
  slices.
- Size regression guards (`unsafe.Sizeof` assertions or generator checks) on
  hot records.

Measure construction time and peak coexistence with the modernc CST, not just
final sidecar size. Differential tests prove every indexed kind, span, child
sequence, token sequence, comment attachment, and walk order matches the
existing CST. Reject the sidecar if it helps SFTPGo but materially regresses
small-file latency — small repositories are a first-class workload.

### Phase 6: Scheduling under CPU and memory budgets

Evaluate ordering and memory limits as one design, after per-file memory
behavior is known:

- [ ] Compare FIFO, largest-first, and work-stealing together with the
  byte-weighted admission limit.
- [ ] Test whether package loading can overlap native checks once CST memory
  pressure is reduced; do not enable overlap by default if it increases peak
  memory or worsens the two-core result.

For each scheduler report wall time, summed work time, peak live bytes/RSS,
worker idle time, and slowest-file completion. Deterministic diagnostics are
required; identical internal completion order is not.

Acceptance:

- Better native-core latency without regressing the fixed two-core median.
- Stable peak memory on repositories with several very large files.
- Identical output at worker counts 1, 2, 4, and the machine default.

## Separate follow-up decisions (outside this plan)

- Semantic/package/type/SSA cache: pursue only if the desired warm full-check
  target requires it. Invalidation must account for Go version, module graph,
  build tags, GOOS/GOARCH, dependency export data, and cross-package changes.
- CST-native formatter (removing the production `go/format` boundary): a
  separate correctness RFC, started only if the post-Phase 4 profile shows
  the residual `go/format` cost justifies owning canonical Go formatting.
  The RFC must define supported Go versions, syntax-conformance inputs,
  comment/directive preservation, semantic-equivalence checking, fuzz
  duration, and the compatibility policy where Strider intentionally differs
  from `gofmt`. Zero corpus changes are necessary but not sufficient.

## Checkpoints after every phase

1. Run `make check`, `make test`, and `make corpus-check`. Most phases require
   zero digest changes: a `make corpus-update` diff is a failed acceptance
   criterion, not a baseline refresh, unless explicitly reviewed.
2. Re-run the end-to-end benchmark matrix.
3. Capture a fresh CPU and allocation profile.
4. Re-rank the remaining phases from the new profile. Do not commit in
   advance to the sidecar, persistent CST serialization, the scheduling
   rewrite, or the CST-native formatter.

## Pull request requirements

One phase per pull request where practical. Every performance PR includes:

1. Before/after benchmark summaries (median, p95, CPU, allocations, GC,
   external peak RSS) against the Phase 0 baseline on the same host.
2. The exact benchmark command and recorded environment.
3. A CPU or allocation profile explaining the improvement.
4. Corpus finding and digest comparisons.
5. Results of the required repository validation.

Do not combine rule changes with performance work.

## Definition of done

Relative to the Phase 0 baseline on the same machine:

- Phases 1–4 meet their relative gates for format-check, full-check, memory,
  and native-syntax CPU.
- Warm unchanged runs meet the mode-specific Phase 2 targets.
- Formatter safety, semantic equivalence, idempotence, and deterministic
  diagnostics remain intact.
- Cold, warm, fixed-core, and native-core results are published and clearly
  labeled, with raw samples retained.
