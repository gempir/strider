# Strider refactor plan — round two

This is a follow-up to `refactor.md`, not a restart of it. The first plan is
fully checked off and materially improved the repository: the public product
now has one `check` command, formatter and fix safety are strong, the large
application and formatter files were split, semantic checks are mostly
colocated with their tests, and the corpus suite protects behavior and
performance.

The remaining debt is different from the debt described in the first plan.
There are few interfaces and the package graph is not generally
over-engineered. The main problems are:

1. the "unified" check catalog still selects and configures checks through
   three registries;
2. syntax checks look independent in the API but are still routed through
   shared, string-keyed behavior functions;
3. behavioral configuration is a closed union repeated across several
   switches;
4. generated documentation, CI, and release workflows do not enforce all of
   the repository's own contracts; and
5. most remaining production code is the check inventory, so future line
   reductions require product judgment, not arbitrary file splitting.

The work below is ordered by correctness and leverage. A phase should earn its
complexity reduction with deleted code, fewer synchronization points, a
smaller contributor workflow, or measured runtime improvement. Moving code
between files is not itself a refactor win.

---

## Audit snapshot (2026-07-23)

- Tracked Go is roughly 50k lines including tests and generated code.
- Handwritten production Go is roughly 27.7k lines. About 19.6k lines (71%)
  are under `internal/checks`; this is where meaningful simplification has to
  happen.
- `internal/cst/zz_nodes_generated.go` is 7,702 generated lines. It should not
  be counted as ordinary maintenance debt.
- The live catalog contains **204 checks**: 97 syntax checks, 106 semantic
  checks, and `format`.
- Built-in severities are 60 error, 129 warning, and 15 note, so the built-in
  warning floor admits 189 checks. The README and generated documentation
  still say 207 and 191.
- This repository's `strider.toml` changes the effective distribution to 53
  error, 75 warning, and 76 note. In other words, the project demotes 61
  checks from the built-in warning/error floor when checking itself.
- The aggregate test statement coverage observed during this audit was 64.8%.
  That number is context, not a proposed quality gate; generated CST code and
  large integration paths make one global threshold misleading.

### What should be preserved

- Formatter equivalence checks, convergence protection, and round-trip fuzzing.
- Atomic multi-file writes and fix validation.
- Deterministic diagnostic output and readable check-inventory goldens.
- The split between cheap concrete-syntax checks and package/type/SSA checks.
- The corpus correctness and performance budgets.
- The use of `x/tools` for packages, inspection, CFG, and SSA, and modernc's
  CST for lossless source work.

### What "simpler" means for this plan

- One normalization and selection pass, not three similar passes.
- One owner for metadata, behavior, options, and documentation facts.
- No string code needed to report from inside a check.
- No option-name switch outside the option schema.
- No exported helper that exists only to make an internal test convenient.
- No new abstraction unless it removes more branches, tables, or duplicated
  code than it introduces.
- No behavior change without golden, corpus, and performance evidence.

---

## SOLID and API assessment

| Principle | Current assessment | Follow-up |
|---|---|---|
| Single responsibility | Most leaf packages are focused. `checks.NewRegistry` still performs discovery, metadata adaptation, normalization, selection, capability derivation, configuration, and construction of two child registries. Syntax behavior groups also mix unrelated checks. | Make the catalog descriptive, selection singular, and each engine consume a prepared execution plan. |
| Open/closed | This is the largest weakness. A new option changes `config.CheckConfig`, presence detection, validation, cloning, core accessors, syntax routing, and docs. A syntax check can require edits across the central catalog and behavior switches. | Make option schemas and check definitions declarative and local. |
| Liskov substitution | There is no broad inheritance hierarchy to fix. The notable contract problem is that an allegedly immutable `workspace.Workspace` is silently consumed by runners calling `File.Release`. | Pick and test one explicit workspace lifetime contract. |
| Interface segregation | There are few interfaces, which is good. Semantic checks nevertheless all implement `Requirements()` even when returning the default, and one configurable check creates a second execution interface for the whole engine. | Keep the base interface minimal and make non-default requirements optional. |
| Dependency inversion | `checks/core` imports the concrete TOML-facing `config` model, and CLI orchestration imports `semantic` for overlay validation. | Make core selection operate on neutral values and keep engine details behind `checks`. |

The goal is not to manufacture one behavioral interface for CST and SSA
checks. Their execution needs are legitimately different. If the only common
contract is metadata, call it a `Descriptor` or catalog entry rather than
claiming that it is a unified executable `Check`.

---

## Phase 0 — Repair gates and remove proven residue

These are small, low-risk changes. Do them before a larger engine migration.

### 0a. Make automation enforce the repository's real contract

- [ ] Fix `.github/workflows/nightly.yml`: its `release` job declares
  `needs: [test, build]`, but the workflow has no `test` job. Add the intended
  test job or remove the nonexistent dependency only if another required gate
  is demonstrably equivalent.
- [ ] Put the canonical local verification sequence behind one Make target
  and use it in CI. Today `.github/workflows/ci.yml` runs `go test` and
  `go vet`, but never runs `make check`.
- [ ] Gate release asset upload on tests. The workflow is triggered only after
  a release is already published; if tests must gate publication itself,
  switch to a tag-push or draft-release promotion workflow.
- [ ] Add macOS and Windows smoke coverage for the filesystem/path-sensitive
  packages shipped on those platforms. Keep the full race job on Linux, but
  do not rely on cross-compilation as the only non-Linux signal.
- [ ] After documentation generation, run `git diff --exit-code` for generated
  files. `bun run build` regenerates catalog data, but CI currently accepts a
  dirty tree, which permits `docs/src/generated/catalog.json` to remain stale.
- [ ] Generate or omit volatile catalog totals in README/docs. Do not copy
  `204` or `189` into several prose files by hand.
- [ ] Add pinned dependency hygiene to CI: `go mod verify`,
  `go mod tidy -diff`, and a scheduled or explicitly versioned
  `govulncheck`. Keep these separate from product checks so failures identify
  their owner.

**Done when:** a deliberately stale generated catalog fails CI, `make check`
is a required CI gate, and nightly/release workflow dependencies are valid.

### 0b. Delete unused and test-only production surface

Static reachability and parameter analysis found concrete cleanup:

- [ ] Delete the unused production functions
  `app.filterDiagnostics`, `semantic.isNamedReceiverType`,
  `semantic.isNilableType`, and `rules.commentDirective`.
- [ ] Remove the unused test helper in
  `semantic/correctness_test_helpers_test.go`.
- [ ] Remove the always-false `fallback` parameter from
  `app.boolOption`; inline or specialize the
  always-`propertySpaceBefore` argument to
  `formatter.layout.markFirstToken`.
- [ ] Delete `syntax.Run`, `syntax.checkFile`, and their private file-reader,
  worker, error, and sorting path. Production calls `AnalyzeTree`; only syntax
  tests call the disk runner. Move those tests to the production execution
  boundary first. This should remove roughly 110 lines.
- [ ] Collapse the three syntax registry constructors to the one production
  shape. Move test conveniences into `_test.go`.
- [ ] Delete child-engine `KnownCodes` state and methods if the unified
  catalog remains their only consumer. Likewise remove test-only
  `semantic.UsesSSA` and `semantic.RequirementsFor`; tests can inspect
  descriptors directly.
- [ ] Delete unused capability API (`CapabilitySource` and
  `Registry.Capabilities`) unless a production scheduler starts using it.
- [ ] Add a lightweight, pinned unused-code check to verification so this
  residue does not accumulate again. Avoid adopting a large lint bundle only
  for this purpose.

**Done when:** every exported production API has a production caller or is an
intentional external contract, and tests no longer keep duplicate execution
paths alive.

### 0c. Fix deterministic input handling

- [ ] Normalize check codes once at configuration/API entry. A config with
  both `format` and `FORMAT` currently overwrites one value according to Go
  map iteration order.
- [ ] Reject duplicate case-folded spellings with a deterministic, sorted
  error. Route `EffectiveCheck`, selection, fixes, and explain/list lookups
  through the same normalization helper.
- [ ] Validate exclusion globs while constructing configuration. `pathfilter`
  currently ignores `doublestar.Match` errors, turning malformed patterns into
  silently ineffective exclusions.
- [ ] Add table tests for uppercase codes, duplicate spellings, malformed
  globs, and stable multi-error ordering.

**Done when:** logically equivalent input spellings behave identically, while
ambiguous or malformed configuration fails before execution.

### 0d. Give diagnostic order one owner

- [ ] Move the byte-for-byte `sortDiagnostics` policy from
  `checks/run.go`, `syntax/syntax.go`, and `semantic/semantic.go` into
  `diagnostic.Sort` or `diagnostic.Compare`.
- [ ] Move the end-offset tie-breaker test with that policy and test all keys:
  file, start offset, code, message, end offset, and severity.

This is a small deletion of about 50 duplicated lines and prevents the three
engines from drifting.

---

## Phase 1 — Build the safety net for structural changes

The current suite is broad, but several important contracts are only tested
indirectly. Add these tests before replacing the registry and syntax engine.

### 1a. Catalog and selection contracts

- [ ] Add direct table tests for `core.Select`: duplicate descriptors,
  case normalization, unknown settings, unknown `--only` entries, severity
  overrides, minimum severity, option validation, stable errors, and input
  ownership/no mutation. `internal/checks/core` currently has no direct
  statement coverage.
- [ ] Replace top-level assertions hardcoding `204` with one readable unified
  inventory golden assembled from the existing syntax and semantic goldens.
  A new check should produce a useful code diff, not a changed integer.
- [ ] Add an option-schema invariant: option names are unique, kinds are
  supported, defaults satisfy validation, configured values round-trip, and
  no declared value is silently ignored.
- [ ] Add a check-author contract helper that validates code uniqueness,
  severity, source ranges, line/column values, edit ordering, `OldText`,
  safety, and automatic-fix uniqueness.

### 1b. Public output contracts

- [ ] Add a full JSON golden/round-trip test covering positions, notes,
  multiple fixes, edits, escaping, omitted/empty fields, and zero findings.
  JSON is an integration API even if its Go package is internal.
- [ ] Add multi-file and multi-package determinism tests that compare complete
  ordered diagnostics under different `GOMAXPROCS` values. Replace the
  current one-file syntax test with a test of the top-level production runner;
  the existing test never enters its worker path.
- [ ] Add named-result and naked-return ownership cases for
  `unclosed-http-response-body` and `unclosed-sql-resource`, including
  replacement of a named result before return.

### 1c. Property and fuzz coverage

- [ ] Fuzz fix edit normalization and application: valid ranges never panic,
  non-overlapping edit permutations produce the same output, overlap policy
  is deterministic, and apply/verify agrees with a small reference model.
- [ ] Fuzz the custom line diff for reconstruction: applying its operations
  must reproduce the new source exactly.
- [ ] Add a bounded CST losslessness fuzz target around parse/token/source
  reconstruction. Keep the existing formatter round-trip fuzz target.
- [ ] Run checked-in fuzz seed corpora through ordinary `go test` in pull
  requests. Reserve mutational `-fuzz` runs for bounded scheduled jobs.

**Done when:** the registry can be rewritten with failures that identify the
broken contract, concurrent output is tested on an actually concurrent path,
and the highest-risk text transformations have property coverage.

### 1d. Cancellation and worker lifecycle

The CLI currently creates cancellation only inside watch mode, after command
setup. `app.Run` has no context, worker runners do not share one cancellation
contract, and `packages.Load` itself has no context hook.

- [ ] Make the application entry point and check/format runners accept one
  caller-owned `context.Context`.
- [ ] Check cancellation before expensive stages, while dispatching/collecting
  worker tasks, and immediately after non-cancellable `packages.Load`.
- [ ] On the first worker error or cancellation, stop admitting work and join
  every started worker before returning.
- [ ] Test pre-cancel, mid-run cancel, first-error cancellation, no blocked
  result send, and goroutine completion. Document the unavoidable
  `packages.Load` cancellation boundary.

---

## Phase 2 — One catalog, one selection, one option schema

This is the central architectural phase.

### Current problem

`checks.NewRegistry` currently:

1. constructs a complete syntax registry to discover syntax metadata;
2. constructs a complete semantic registry to discover semantic metadata;
3. copies both into metadata-only `catalogCheck` values and mutates
   capabilities on the copies;
4. runs top-level `core.Select`;
5. splits the selected codes and settings back into two categories; and
6. constructs both child registries, each of which selects again.

The same policy is therefore applied up to three times. A check's `Meta`
also has different capability data depending on which registry returned it.
`Meta.Capabilities` is a raw `uint8`, even though it is scheduler data rather
than explanatory metadata.

### 2a. Replace registry layering with a prepared execution plan

- [ ] Have each engine expose an immutable descriptor catalog without
  constructing a configured registry.
- [ ] Build global uniqueness and presentation metadata from those
  descriptors once.
- [ ] Normalize, validate, resolve severity, and select once at the top-level
  boundary.
- [ ] Produce a prepared execution plan containing the already-selected
  concrete syntax and semantic checks, effective severities, exclusions, and
  typed options. Engines must not reselect or reinterpret configuration.
- [ ] Delete `catalogCheck`, `selectedCheckCodes`, `selectedSettings`, child
  known-code maps, and duplicate constructor/selection paths.
- [ ] Move engine kind, requirements, and scheduling capabilities out of
  user-facing `Meta`, or at minimum make capability values typed and immutable.
- [ ] If the common executable interface still contains only `Meta()`, rename
  it to `Descriptor`/`CatalogEntry`. Do not force CST and SSA execution behind
  an `any`-based interface merely to preserve the word "unified".

**Acceptance criteria:**

- Exactly one call site owns code normalization, unknown-code errors,
  severity resolution, and option validation.
- Engine execution accepts a prepared plan and cannot select checks again.
- Metadata is immutable and has the same value from every API.
- Adding a third engine requires one catalog adapter, not another selection
  layer.

### 2b. Replace `CheckConfig`'s option-name union

Adding one integer or string-list option currently changes all of:

- fields in `config.CheckConfig`;
- `ConfiguredOptions`;
- negative-value validation;
- cloning;
- `core.IntOption` or `core.StringsOption`;
- syntax's code-specific `limits`/characters/import routing; and
- the documentation generator's hardcoded configurable-check list.

That is the clearest open/closed violation in the repository.

- [ ] Keep common policy (`severity`, `excludes`) explicit, but decode
  behavioral settings into a generic typed option map.
- [ ] Let each check's `OptionSpec` own name, kind, default, bounds, and help
  text. Adding an option *name* must not require a central field or switch.
  Adding a genuinely new option *kind* may update the small schema codec.
- [ ] Separate TOML decoding from catalog binding so the dependency direction
  remains `config -> neutral values -> checks`; `checks/core` must not depend
  on the concrete TOML configuration struct.
- [ ] Validate unknown keys, types, ranges, and duplicate case-folded names
  deterministically against the selected descriptor.
- [ ] Configure immutable check instances or give every check the same
  read-only typed option accessor. Remove syntax's per-code `limits` switch
  and semantic's one-off `RunConfigured` interface.
- [ ] Derive option documentation and default rendering directly from
  `OptionSpec`; delete the generator's parallel configurable-check table.

**Acceptance criteria:**

- Adding another `max-*` option changes one check definition and its test, not
  `config`, `core`, syntax routing, and docs code.
- Every supported option has one schema declaration and one validation path.
- Version-1 TOML remains strict and compatible except for previously
  ambiguous case-duplicate input, which now fails clearly.

### 2c. Complete the package boundary

- [ ] Stop importing `semantic` from `internal/app` merely to pass
  `semantic.ValidateOverlay`. Expose package-overlay validation through the
  unified `checks` execution boundary or a focused source/package-loading
  package.
- [ ] Re-evaluate the name `core` after selection is simplified. If it owns
  only descriptors and selection, `catalog` or `selection` is clearer; if it
  becomes trivial, delete the package instead of renaming it.

---

## Phase 3 — Make syntax checks genuinely independent

This has the highest contributor and runtime payoff, but also the widest
behavior-preservation risk.

### Current problem

The syntax API is modular only at the surface:

- all 97 entries are declared in a 1,186-line `rules/catalog.go`;
- 35 shared behaviors route those entries;
- 39 `active("code")` checks and roughly 109 `report("code", ...)` calls
  reconnect behavior to metadata with strings;
- reporting silently drops a finding when its string code differs from
  `activeCode`;
- 19 checks use `functionBehavior`, and `timeNamingBehavior` also invokes the
  function path, so `functionFacts` can walk the same function body about 20
  times; and
- 10 checks use `callBehavior`, causing the broad `checkCall` switch to run
  repeatedly for each call.

This is difficult to extend, easy to mistype, and needlessly repeats work.

### Target design

- [ ] Give each syntax check one implementation unit that owns its metadata,
  typed interests, inspection callback, options, and any run-local state.
  Shared parsing/fact helpers are encouraged; shared helpers must not route
  behavior by check code.
- [ ] Bind a `Pass` or reporter to the current descriptor. Inside a check,
  `Report(...)` must infer code and effective severity. Remove code strings
  from all reporting calls.
- [ ] Replace free-form node-kind strings and unchecked assertions with typed,
  generated node-kind constants or typed registration helpers.
- [ ] Model file-start/file-finish hooks explicitly rather than as pseudo node
  kinds such as `<file>` and `<finish>`.
- [ ] Build expensive derived facts once per relevant node and expose them
  read-only. At minimum, a function body's `functionFacts` must be computed
  once no matter how many checks consume it.
- [ ] Replace the reflection/map state keyed by `activeCode` with state owned
  by the check instance or one execution context.
- [ ] Merge `internal/checks/syntax/rules` into `syntax`, or rename the package
  after its boundary is clear. The current `rules` name and `builtinchecks`
  alias contradict the repository's one-noun policy.

### Migration strategy

- [ ] Introduce the new descriptor/runner behind a temporary adapter so old
  and new definitions share one selected plan.
- [ ] Migrate one behavior family at a time (simple declarations, calls,
  functions, control flow, then stateful/file-level checks).
- [ ] For every family, compare exact diagnostics, ranges, fixes, and corpus
  output before deleting the old behavior.
- [ ] Delete each old behavior path immediately after its last check migrates;
  do not leave two permanent frameworks.
- [ ] Add execution-count benchmarks or test instrumentation proving shared
  facts are computed once. Compare all-check wall time and allocations before
  and after on the corpus.

**Acceptance criteria:**

- A syntax check can be understood from its own implementation and focused
  test.
- There are zero `active("...")` gates and zero report calls containing a
  check code.
- There is no central switch from code to behavior or option.
- A typo cannot silently suppress all findings from a check.
- Adding a check requires two authored files (implementation and test);
  registration, inventory, and documentation are generated or updated by one
  declarative source.
- All current syntax goldens and corpus outputs remain stable unless a
  separately reviewed behavior fix says otherwise.

Avoid replacing one god switch with 97 verbose structs containing boilerplate.
Tiny constructors for genuinely identical patterns are useful if the check
still owns its metadata and routing remains compile-time safe.

---

## Phase 4 — Reduce semantic engine ceremony

The semantic side is structurally healthier: behavior generally lives with
the check, and type/SSA requirements are real. Its remaining gains are smaller
and lower risk.

- [ ] Remove per-check default requirement methods. Roughly 60 checks spend
  about 300 lines returning the same type-analysis requirement. Preserve
  safety with explicit descriptor constructors/registration for type and SSA
  checks; non-default facts and features remain explicit descriptor data.
  Do not use an optional interface that could silently register an SSA check
  as type-only.
- [ ] Remove `configurableCheck`/`RunConfigured`, which exists for only
  `interface-method-limit`. Use the common immutable configuration mechanism
  from Phase 2.
- [ ] Remove the duplicate `firstArguments` fact storage; derive it from the
  existing argument-slice map.
- [ ] Build the static-call fact index in one pass unless profiling proves
  that a preliminary capacity-count pass wins enough to justify itself.
- [ ] Replace `fmt.Sprintf`-built finding de-duplication keys with a comparable
  struct containing the actual fields.
- [ ] Add the multi-package concurrency determinism contract from Phase 1 to
  the semantic worker path.
- [ ] Measure task queue memory for packages × selected checks. Bound buffers
  if it is material, but do not introduce a generic worker-pool framework
  merely because two loops look similar.

**Acceptance criteria:**

- A simple semantic check implements only metadata and `Run`.
- Only checks with non-default needs declare detailed requirements, while
  every catalog entry still chooses its analysis stage explicitly.
- There is one configuration execution path.
- Fact construction and de-duplication use typed data rather than redundant
  maps or formatted strings.

Do not split `semantic.go` solely to make files shorter. Split only where the
resulting loader, fact builder, or executor has a stable independent contract.

---

## Phase 5 — Strengthen testability, helpers, naming, and dependencies

### 5a. Remove process-global test coupling

- [ ] Pass configuration discovery a starting directory instead of having it
  call `os.Getwd`. Do the same for source/package discovery where the caller
  already knows the root.
- [ ] Replace mutable package globals in `filewrite`
  (`createTemporary`, `renameFile`) with a small injected operations value on
  the unexported implementation; keep the public API simple.
- [ ] Remove remaining `os.Chdir` test setup in app/config tests.
- [ ] Add `t.Parallel()` selectively to pure unit/table tests after global
  state is gone. Do not parallelize expensive package-loading tests merely to
  increase the count.
- [ ] Split the 1,469-line syntax integration test by check or behavior family
  as Phase 3 migrates. Prefer exact positive and adversarial-negative goldens
  over "the code appeared at least once".

### 5b. Make workspace ownership explicit

`Workspace` is documented as immutable, but `checks.Run` and formatting defer
`File.Release`, after which later source/CST access fails.

- [ ] Choose one contract: either a workspace is explicitly single-use and
  closable, or runners are read-only and the owner releases the workspace.
- [ ] Prefer one workspace-level `Close`/`Release` over hidden per-engine
  consumption if resource release is required.
- [ ] Add contract tests for repeat-run behavior, access after close, and
  idempotent close.

### 5c. Use the standard library and existing modules where they simplify

- [ ] Replace the hand-maintained predeclared-identifier table with
  `go/types.Universe` where semantic identity is needed, and
  `token.IsIdentifier` where lexical validity is needed.
- [ ] Inline the one-line UTF-8 decode wrapper.
- [ ] Parse module paths in `formatter/module_cache.go` with
  `golang.org/x/mod/modfile.ModulePath` rather than a line scanner. `x/mod` is
  already transitive; make it direct when imported.
- [ ] Evaluate replacing the roughly 190-line custom Myers/unified-diff code
  in `app/format.go` with a small, stable Go library. Add the dependency only
  if it preserves exact output, has acceptable maintenance/security posture,
  and deletes substantially more code than the adapter adds. Otherwise move
  the algorithm behind a focused tested package and keep it.

All current direct production dependencies are justified:

- BurntSushi TOML provides strict config decoding;
- doublestar implements documented glob semantics;
- `x/tools` supplies packages, inspector, CFG, and SSA;
- modernc's parser supplies the lossless CST needed by formatting and syntax
  checks.

Do **not** add Cobra/pflag, an assertion framework, or a blanket lint
aggregator merely to make the stack look conventional. The current CLI is
small enough for `flag`; ordinary Go tests are readable; and a wholesale
rewrite as `go/analysis.Analyzer` values would complicate the embedded
formatter/CST pipeline. If external third-party checks become a real product
goal, first evaluate an adapter for `*analysis.Analyzer` at build time rather
than inventing a runtime plugin ABI.

### 5d. Finish the naming sweep after boundaries settle

- [ ] Remove or rename `syntax/rules` and the `builtinchecks` import alias as
  part of Phase 3, not in a standalone churn commit.
- [ ] Rename stale `lint_test.go` / `analyze_test.go` files and failure text to
  syntax/semantic or unified-check terminology.
- [ ] Remove research-session names from test helpers.
- [ ] Keep old documentation URLs and hidden CLI aliases where compatibility
  has value; internal naming consistency does not justify breaking links.

---

## Phase 6 — Reassess the check inventory as product surface

This is the only phase likely to produce another large line-count reduction.
It requires evidence and product judgment, not a mechanical complexity limit.

The repository demotes 61 built-in warning/error checks to note in its own
configuration. That does not prove those checks are bad, but it is a strong
signal that default severity and trusted signal need review.

- [ ] Produce a catalog report containing implementation LOC, engine stage,
  corpus runtime, corpus finding count, project effective severity, fix
  availability/safety, and overlap with `go vet`, Staticcheck, or another
  established analyzer.
- [ ] Review the most complex heuristic implementations first:
  `failed-assertion-shadow-read`, `invalid-printf-call`,
  `append-to-sized-slice`, `context-cancel-in-loop`,
  `possible-nil-dereference`, `slice-preallocation`, and
  `unclosed_resources.go`.
- [ ] For each check, choose keep, simplify, demote, or delete. Record a short
  rationale and false-positive boundary so the same debate is not repeated.
- [ ] Require retained complex checks to have unique actionable value,
  focused positive tests, adversarial negative tests, reviewed corpus signal,
  and a default severity the project can defend.
- [ ] Prefer deletion or a documented conservative approximation over another
  configuration knob or a hidden mini dataflow engine.
- [ ] Remove only filler metadata such as "Default: enabled". Preserve
  behavioral contracts such as thresholds, recognized APIs, and character
  policies, and generate configurable defaults from `OptionSpec`. Allow
  summary fallback or require a genuinely useful rationale.
- [ ] Exercise documented examples where practical: a bad example should
  trigger its check and a good example should remain clean. Mark fragments
  that cannot be compiled so exceptions are explicit rather than silently
  skipped.

**Acceptance criteria:**

- Every built-in warning/error has a documented signal rationale.
- Every repository severity override is intentional and reviewable.
- No complex check is retained only because it already exists.
- Catalog/docs statistics are generated from the retained inventory.

There is deliberately no target number of checks or maximum file length. A
well-tested printf parser can be long; a short noisy check can still be a bad
product.

---

## Phase 7 — Right-size watch caching with measurements

Watch mode currently uses both a 301-line workspace snapshot/LRU cache and a
233-line whole-result `checks.Session`. The session fingerprints every file
and ancestor `go.mod`, deep-clones concrete diagnostics and candidates, but
semantic analysis still runs fresh each iteration.

- [ ] Benchmark unchanged, one-file-changed, and dependency-changed watch
  iterations on a small and a large repository. Record wall time,
  allocations, retained heap, and cache hit/miss data.
- [ ] Decide in advance what benefit justifies the result cache. If the
  dominant package load remains uncached and the second layer has no material
  win, delete it and retain one previous workspace generation plus diagnostic
  comparison.
- [ ] If both caches earn their keep, give each a non-overlapping documented
  responsibility and one shared fingerprint/module-boundary implementation.
- [ ] Apply the workspace lifetime decision from Phase 5 so cache eviction,
  close, and reuse are explicit.

Do not reopen the old fsnotify-versus-polling debate or formatter convergence
loop without new profiles. `refactor.md` already made those decisions. This
phase is about whether the *second result cache* pays for its code.

---

## Contributor workflow target

`CONTRIBUTING.md` currently explains validation but not how to add a check.
The old plan's "exactly two files" rule is not true today: a semantic check
also touches the central catalog, inventory golden, and a pre-existing docs
page; a syntax check navigates even more central routing.

- [ ] Add a short "authoring a check" guide with one minimal syntax example
  and one semantic example.
- [ ] Provide a small scaffold/update command, or generate registration,
  inventory, and docs from per-check descriptors.
- [ ] Add and document one golden-update target. The current
  `STRIDER_UPDATE_GOLDEN=1` mechanism exists only inside syntax and semantic
  test helpers and is invisible to contributors.
- [ ] Make the command fail if a code collides case-insensitively, metadata is
  incomplete, an option schema is invalid, or a docs page cannot be produced.
- [ ] Add a scaffold smoke test that creates a temporary check fixture and
  verifies the catalog/test/doc update path.

The target is **two authored files**: implementation and focused test.
Generated inventory/docs changes may appear in a commit, but the contributor
should not hand-edit parallel registries or routing switches.

---

## Sequencing summary

| Phase | Theme | Risk | Expected payoff |
|---|---|---:|---|
| 0 | CI correctness, deterministic input, dead code, shared sorting | Low | Fix broken nightly dependency; remove duplicate/test-only surface |
| 1 | Contract, output, concurrency, and fuzz guardrails | Low-medium | Makes structural migration failures local and explainable |
| 2 | One catalog/selection and descriptor-driven options | Medium | Removes the main OCP/DIP violations and several hundred lines of plumbing |
| 3 | Real independent syntax checks | Medium-high | Best contributor experience and largest avoidable runtime repetition |
| 4 | Semantic defaults and fact simplification | Low-medium | Roughly 300 lines of boilerplate plus typed, cheaper internals |
| 5 | Testability, helpers, naming, dependency policy | Low-medium | Less global state, clearer ownership, smaller helper code |
| 6 | Evidence-based inventory review | Product risk | The only credible path to a large net LOC reduction |
| 7 | Measured watch-cache decision | Medium | Either proven performance or deletion of an unearned cache layer |

Phases 0 and 1 are prerequisites. Phase 2 should precede Phase 3 because the
new syntax checks need the final catalog and option APIs. Phase 4 can proceed
after Phase 2 in parallel with the syntax migration. Inventory decisions
should be incremental and separately reviewable.

---

## Validation protocol

After every task:

```text
make check
make test
```

After every completed phase:

```text
make corpus-check
make corpus-update
```

Also apply the phase-specific proof:

- Registry/config changes: direct selection tables and generated-doc
  cleanliness.
- Syntax/semantic changes: exact diagnostic/fix goldens and multi-worker
  determinism.
- Performance claims: before/after benchmarks and corpus budget comparison.
- Dependency changes: `go mod tidy -diff`, `go mod verify`, vulnerability
  scan, and license/maintenance review.
- Cross-platform filesystem/path changes: Linux, macOS, and Windows smoke
  tests; keep race testing on Linux.

`corpus-update` is not evidence that changed behavior is correct by itself.
Review any output delta and state whether it is an intended product change.

## Definition of done

This follow-up is complete when:

1. CI, nightly, release, and generated-documentation gates enforce the same
   validation promised to contributors.
2. There is one canonical code-normalization, option-validation, and
   selection path.
3. Engine metadata is immutable and engine execution consumes one prepared
   plan.
4. Syntax checks no longer use string code routing, active-code report
   filtering, or repeated family dispatch.
5. Default semantic checks do not implement empty requirements ceremony.
6. Adding a check or behavioral option has one declarative owner and no
   parallel central switch.
7. Public JSON, diagnostic ranges/fixes, concurrent determinism, and generated
   docs have explicit contract tests.
8. Process-global CWD and file-operation mutation no longer force the suite to
   remain serial.
9. Every retained complex check and cache layer has measured, documented
   value.
10. Required checks, tests, corpus validation, and corpus update all pass.
