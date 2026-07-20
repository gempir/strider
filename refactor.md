# Strider refactor plan

A review of the codebase (excluding `docs/`) against the goals: code quality, a
common check API, naming consistency, readability, simplicity, focused
packages, SOLID, and questioning features that don't earn their keep.

The codebase is ~53k lines of Go. The leaf packages (`diagnostic`, `baseline`,
`filewrite`, `pathfilter`, `ui`, `cst`) are mostly small and well-tested, and
the safety engineering (formatter verification, atomic writes, fix
validation) is above average. The structural debt is concentrated in three
places:

1. **Two incompatible check APIs** (`syntax` vs `semantic`) with copy-pasted
   registry plumbing on top.
2. **`internal/app`** — a hand-rolled CLI framework, two hidden duplicate
   commands, and engine code mixed into command handlers.
3. **Accretion artifacts** — files named after work sessions
   (`research_*.go`, `moved_syntax_rules.go`), dead code, migration
   scaffolding, and fake configuration knobs.

The plan is ordered so that each phase stands alone: deletions first, then the
unified check API (the biggest win), then naming, then package boundaries.
Run `make check && make test` after every step; run `make corpus-check` and
`make corpus-update` when a phase completes.

---

## Phase 0 — Delete dead weight (no behavior change)

Cheap, mechanical, and shrinks everything that follows.

- [x] **Delete `UsesCST` + `cstRuleCodes`** in
  `internal/checks/syntax/rules/cst_engine.go:11-149`. `UsesCST` has zero
  callers; `cstRuleCodes` is a 96-line hand-maintained copy of the catalog
  that exists only to support it. Migration scaffolding.
- [x] **Delete `UnsupportedError` / `ErrUnsupported`** in
  `internal/formatter/formatter.go:21-68`. Never constructed, never returned,
  never tested.
- [x] **Delete or hide the `lint` and `analyze` commands**
  (`internal/app/app.go:692-758`, `848-912`, plus their list/explain helpers).
  They are ~450 lines that duplicate `check --only ...`, are absent from
  `usage()` and the README, and triple every rendering function
  (`listLintRules`/`listAnalyzeRules`/`listChecksInRegistry`,
  `explainLintRule`/`explainAnalyzeRule`/`explainCheck`). One product command
  (`check`) is simpler to explain, test, and maintain.
- [x] **Remove `formatter.html` (125 KB) from the repo root.** Nothing
  references it; it is a pasted design document. Move the content into
  `docs/` if it still has value, otherwise delete.
- [x] **Delete unused exported API:** `baseline.Apply` and
  `baseline.ApplySelected` (only `ApplyCatalogSelection` is used,
  `internal/app/app.go:1121`), `ui.Palette.Enabled()`,
  `workspace.Workspace.Generation()` (test-only), `cst.Tree.Source()`
  (test-only). Unexport or remove.
- [x] **Collapse `diagnostic.Safety` to two levels or make three meaningful.**
  `fix.allowed` (`internal/fix/fix.go:372-374`) treats `PotentiallyUnsafe`
  and `Unsafe` identically — the distinction is dead granularity.
- [x] **Remove fake configuration.** `formatter.Options.MaxBlankLines` and
  `ExistingLineBreaks` accept exactly one legal value each
  (`formatter.go:162-166`); `config.validate()` likewise requires fixed
  values for `max-blank-lines`, `existing-line-breaks`, and
  `alignment.declarations` (`config.go:245-265`). Delete the knobs; a
  formatter with no choices should have an options struct that reflects
  that (`PrintWidth` is the only real option).
- [x] **Fix the contradictory severity tables** in
  `internal/checks/syntax/rules/catalog.go`: `empty-conditional-block`,
  `ineffective-pointer-copy`, and `inefficient-map-lookup` appear in *both*
  the warning and error sets (`catalog.go:538/602`, `550/603`, `554/604`);
  the warning entries are silently dead. (This table disappears entirely in
  Phase 1, but fix the bug first if Phase 1 is delayed.)

## Phase 1 — One check API (the core of this refactor)

Today there are two rule contracts for the same product concept:

```go
// syntax/rules/model.go:20 — metadata only; behavior lives in a monolithic engine
type Rule interface { Meta() Meta }

// semantic/model.go:25 — self-contained
type Rule interface { Meta() Meta; Run(*Pass) }
```

The `Meta` struct is byte-for-byte identical in both packages (and duplicated
a third time as `checks.Meta` in `internal/checks/model.go`). The
`Registry`/`RegistryOptions`/severity plumbing is copy-pasted between
`syntax/syntax.go:33-195` and `semantic/registry.go:167-371`, down to
identical error strings. `internal/checks/registry.go` then wraps both with a
third layer of glue and per-category settings routing.

The semantic package's shape is the right one: **one rule = one file =
metadata + behavior + requirements, registered in one catalog line.** The
syntax package should adopt it.

### 1a. Shared foundation

- [x] Create a single `checks.Meta` (already exists in
  `internal/checks/model.go:30`) and make `syntax` and `semantic` use it
  instead of their private copies. Same for the `Rule` base interface:

  ```go
  package checks

  type Meta struct { Code, Summary, Explanation, GoodExample, BadExample string; DefaultSeverity diagnostic.Severity }

  type Check interface {          // pick ONE noun — see Phase 2
      Meta() Meta
  }
  ```

- [ ] Extract the duplicated registry plumbing (code normalization,
  unknown-code errors, severity resolution, `Only` filtering, excludes,
  minimum-severity gating) into one generic helper in `internal/checks`.
  `syntax.Registry` and `semantic.Registry` become thin wrappers, or ideally
  disappear into a single `checks.Registry` that owns all selection while the
  sub-packages only export their catalogs.
- [x] Normalize codes **case-insensitively everywhere** — semantic does
  (`registry.go:283-291`), syntax matches exactly (`catalog.go:635-641`), and
  `explainLintRule` was case-sensitive while its siblings were not.
- [ ] Collapse the three registry constructors in semantic
  (`NewRegistry` / `NewRegistryConfigured` / `NewRegistryWithOptions`,
  `registry.go:256-272`) to one.

### 1b. Give syntax rules real behavior

The syntax engine currently violates OCP by construction: rules are anemic
metadata, behavior is keyed by string codes inside a god object
(`cstAnalyzer`, 24 fields, `cst_engine.go:118-144`), dispatched by a 180-line
type switch (`cst_engine.go:259-441`) that would fail the package's own
`cyclomatic-complexity` and `function-length` rules. A rule's identity is
spread across **five hand-synced tables** (catalog spec, examples map, two
severity sets, plan flags, plus the check body); adding one rule touches ~7
files. In semantic it takes 2.

- [ ] Define a syntax-side pass, mirroring semantic:

  ```go
  type SyntaxCheck interface {
      checks.Check
      Interests() []NodeKind       // which CST node kinds this check wants
      Inspect(pass *Pass, node cst.Node)
  }
  ```

  The engine keeps the single shared CST traversal (that part is good) but
  builds its dispatch table *from the checks' declared interests* instead of
  the hand-maintained 33-boolean `cstExecutionPlan` (`cst_plan.go:10-44`) —
  which is mostly 1:1 renames of `enabled[code]` anyway.
- [ ] Merge the five tables into **one declaration per rule**: `Meta`
  (including good/bad examples and default severity) lives on the rule, as
  `coreCatalog` already demonstrates (`catalog.go:11-82`). Delete
  `extendedCatalog`, `extendedExamples`, `extendedWarningSeverities`,
  `extendedErrorSeverities`, and the panic-at-startup examples lookup
  (`catalog.go:670`).
- [ ] Delete `checkDefaults` (`cst_engine.go:230-257`) — a duplicate
  dispatcher maintained as a fast path for the 7 core rules. One dispatcher.
- [ ] Remove double gating: check bodies test `a.enabled[code]` and
  `report()` re-tests it (`cst_engine.go:444`). With per-check dispatch,
  neither is needed, and a typo'd code can no longer silently drop findings.
- [ ] Split rule state out of the engine. Per-rule accumulators
  (`receiverNames`, `marshalKinds`, `repeatedLiterals`, `publicStructs`, ...)
  belong to the rules that use them, not to the shared traversal object.
- [ ] Move real explanations onto extended rules or accept shorter metadata —
  the current `Explanation = Summary + ". Default: ..."` synthesis
  (`catalog.go:672`) creates two metadata quality tiers.
- [ ] Behavioral limits (`max-lines`, `max-parameters`, ...) are currently
  stated in **three places** (registry switch `syntax.go:201-221`, check-body
  defaults like `a.limit("max-parameters", 8)`, catalog prose). Each check
  should declare its options and defaults once; the registry validation table
  `supportedBehavioralOptions` (`checks/registry.go:15-41`) and the config
  union struct (Phase 4) derive from that single source.

### 1c. Reduce semantic boilerplate

112 rules × ~10-line `Meta()` methods ≈ 1,100 lines of ceremony, plus 61
hand-rolled `ast.Inspect` full-file walks and a triple-nested SSA loop
repeated per rule.

- [ ] Add `Pass.ReportPos(token.Pos, string)` and delete the `positionNode`
  hack (defined in `invalid_regexp.go:15`, used by 7 SSA rules) — SSA rules
  wrap positions in a fake `ast.Node` today only because `Report` demands one.
  This also removes the main justification for the
  `FactCallArguments`/`FactFirstCallArgument` bitflags (see 1d).
- [ ] Provide one shared typed-node inspector (à la
  `x/tools/go/ast/inspector`) so typed rules declare node interests instead
  of each walking every file. This mirrors the syntax-side `Interests()` and
  makes the two APIs feel like one.
- [ ] Add `isNamedType(t types.Type, pkgPath, name string) bool` and the
  common receiver-unwrap helper in one place; delete the dozens of local
  re-derivations (`isTimeValue`, `isTestingTType`, `infallibleWriterType`,
  `isSyncWaitGroupMethod`, ...).
- [ ] Deduplicate `inspectFunctionBody` / `inspectFunctionBodyNode` /
  `inspectParallelTestBody` — three byte-identical copies
  (`nil_error_returns.go:141-153`, `:203-215`,
  `research_additional_checks.go:294-306`).
- [ ] Move rule-specific state off `Pass`: `maxMethods` exists for exactly
  one rule (`model.go:43`), `deprecatedObjects`/`deprecatedPackages` for one
  more. Per-check options should flow through the check's own configuration,
  not widen the shared pass for everyone (ISP).

### 1d. Right-size the requirements/facts system

The `Requirements{Stage, Facts, SSAFeatures, staticCallPackages}` plan
compiler is sound in concept — skipping SSA when no SSA rule is selected is a
real win — but over-built for its 7 consumers and enforced by runtime panics
in four places (`facts.go:57,70`, `registry.go:223,231`).

- [ ] Keep the `Types` vs `SSA` stage split and `SSAFeatureGlobalDebug`.
- [ ] Declare requirements **on the rule** (a method or struct field), not in
  the central catalog, so a rule body and its needs cannot drift.
- [ ] Replace panic-based invariants with a single unit test over the static
  catalog.
- [ ] Fold `FactFirstCallArgument` into `FactCallArguments` (the distinction
  saves one slice allocation) and re-evaluate whether the fact system
  survives `ReportPos` at all.
- [ ] Fix the case-convention split inside `registry.go`
  (`requirementsByCode` lowercase vs uppercase normalization).

### 1e. Simplify the top-level `checks` wrapper

With 1a–1d done, `internal/checks/registry.go` no longer needs to route
settings into two differently-shaped sub-registries, duplicate `Meta`
fields field-by-field (`registry.go:241-278`), or special-case `format` as a
pseudo-check quite so manually. Target: `checks` owns *one* catalog (format +
syntax + semantic), one settings map, one capability computation.

## Phase 2 — Naming consistency

Pick **one noun: "check."** It is the product word (`strider check`,
`strider.toml [checks.<code>]`). Today "rule", "check", "lint", "analysis",
and "analyzer" are used interchangeably:

- [ ] Types/identifiers: `Rule`→`Check`, `RuleConfig`→`CheckConfig`,
  `ruleCatalog`→`checkCatalog`, `builtinrules` import alias, `lintFile`,
  `validateConfiguredRules("lint", ...)`, `cstAnalyzer`, error strings
  `"unknown lint rule(s)"` vs `"unknown analysis rule(s)"` → `"unknown
  check(s)"` (matching `checks/registry.go:106`).
- [ ] CLI: `check --list-checks` with `--list-rules` kept as a hidden alias.
- [ ] **Rename session-artifact files by content, one check per file:**
  - `semantic/research_style.go` → split into `blank_identifiers.go`,
    `task_comment.go`, `doc_comment_period.go`, etc.
  - `semantic/moved_syntax_rules.go` → `time_value_equality.go`,
    `waitgroup_go_call.go`, `range_value_capture.go`.
  - `research_correctness_test.go` / `research_performance_test.go` /
    `research_additional_checks*.go` → per-check test files colocated with
    what they test.
- [ ] **Give shared helpers a home.** Package-level traversal helpers hide in
  `nil_error_returns.go:107-153`; `positionNode` in `invalid_regexp.go:15`;
  `normalizeGoVersion` in `leaky_time_tick.go:67`; `pluralSuffix` in
  `moved_syntax_rules.go:265`. Move to `semantic/helpers.go` (or the shared
  inspector package from 1c). Same in syntax: unify the three coexisting
  helper prefixes (`checkConcrete*`, `concrete*`, `cst*`) and drop the
  redundant `cst_` file-name prefix inside a package that is entirely about
  the CST.
- [ ] `pathfilter.Matches` returns true when a path is **excluded**; every
  call site reads `if !pathfilter.Matches(...)` to keep a file. Rename to
  `pathfilter.Excluded(...)`.
- [ ] Formatter: rename `concrete_printer.go` identifiers to drop the
  `concrete` stutter; `Session` → something honest (it holds only a module
  cache); `PreviewTree` → a name that says "unverified"
  (`FormatTreeUnverified` or `Verify bool` option);
  `validateConcreteSyntax` cannot fail and doesn't validate — rename or
  inline; fix the `ExprCaseClause = gc.ExprCaseClauseListNode` alias
  (singular name for a list node, `cst/cst.go:51`).
- [ ] `stringsHasArguments` (`cst_control_rules.go:167`) has nothing to do
  with the `strings` package and hand-rolls `strings.HasPrefix` — rename and
  simplify.
- [ ] Standardize error message style: lowercase, one quoting convention for
  enums (`--color must be auto, always, or never` vs config's quoted
  variant), one phrasing per condition (`"--watch cannot update a baseline"`
  vs `"watch mode cannot update a baseline"`), consistent
  operation-context prefixes.
- [ ] Import aliases: `checkengine`/`fixengine` vs bare `syntax`/`semantic`/
  `formatter` in `internal/app` — pick bare names.

## Phase 3 — Split `internal/app` (SRP for the CLI)

`app.go` is 1,160 lines with at least eight responsibilities. After Phase 0
removes `lint`/`analyze`, split what remains:

- [ ] `app/flags.go` — the flag micro-framework (`boolOption`,
  `stringOption`, `varOption`, `flagWasSet*`, `parseCommandFlags`,
  `printFlagDefaults`, `stringList`). Consolidate the **three copies** of the
  alias map (`parseGlobalOptions` app.go:209-216, `checkCommandAliases()`
  app.go:382-395, literal in check.go:41-56) into one table. Alternatively,
  adopt a tiny CLI library and delete the framework — but a single
  table-driven layer is enough and keeps zero deps.
- [ ] `app/format.go` — `runFormat` and stdin/path handling.
- [ ] Move `formatFiles` (worker pool, app.go:508-574) next to the formatter
  or into a small orchestration helper — engine code doesn't belong in
  command handlers.
- [ ] **Delete `atomicWriteBatch`/`stageFile`** (app.go:601-655) and use
  `internal/filewrite` for `fmt --write`. Today the repo has two atomic-write
  implementations and the weaker one (no stale check, no symlink handling)
  serves `fmt` while the strong one serves `check --fix`.
- [ ] Replace the pairwise mutual-exclusion wall in `runCheck`
  (check.go:79-107) with a small declarative conflict table, and extract the
  fix/baseline/watch flows out of the 230-line `runCheck` into focused
  functions. The `printCommandError(...); return exitError` pattern appears
  ~14 times in one function — a `run() error` + single error-printing wrapper
  removes all of them.
- [ ] Fix `fmt --diff`: `printDiff` (app.go:657-668) emits the whole old file
  as `-` and the whole new file as `+`. Implement real hunks or drop the flag.
- [ ] Fix the `--config`/`--no-config` conflict check that only runs in the
  `default:` parse arm (app.go:224-227).
- [ ] Unify the excludes semantics: `lint` filtered **files** before running
  while `analyze` filtered **diagnostics** after (app.go:817 vs 969). After
  deleting both commands, make sure `check` has one documented behavior for
  package-level checks (filter diagnostics, since excluded files can still
  affect package analysis — but say so).

## Phase 4 — Package boundaries and de-duplication

- [ ] **One "is generated" implementation.** `source.IsGenerated`
  (discover.go:123-141) and `workspace.generatedSource` (cache.go:306-319)
  are logic-duplicates on different input types. Put one `[]byte`-based
  function in `source` and use it from both open paths; also fixes the drift
  where `workspace.Open` and `workspace.Cache.Open` apply generated-skipping
  differently.
- [ ] **One severity→style mapping.** `app.colorSeverityText`
  (app.go:999-1010) ≡ `report.styledSeverity` (text.go:242-253). Let
  `report` (or `ui`) own it.
- [ ] **One source-line cache.** `report/text.go:208-223` and
  `report/html.go:400-409` re-implement the same read-file-split-lines cache.
- [ ] **Collapse the report wrappers.** `checks/report.go`,
  `syntax/report.go`, `semantic/report.go` are three ~30-line files
  delegating to `internal/report`, differing only in an HTML title. Export
  `report.JSON(w, diags)` and delete all three — presentation code doesn't
  belong in analysis packages.
- [ ] **Fix the `fix` package layering inversion.** `internal/fix` imports
  `internal/checks/semantic` for overlay type-checking (fix.go:13, 334).
  Inject a `Validate func(paths, sources) error` instead — an "apply edits"
  package must not depend on a specific analysis engine.
- [ ] **Address the path-aliasing symptom.** `fix.Capture` registers six
  alias strings per file (fix.go:152-180) because diagnostics carry free-form
  path strings. Standardize: diagnostics carry root-relative slash paths,
  produced by one function, consumed everywhere. Then `fix` and `filewrite`
  can share one "resolve + identity-check" primitive instead of duplicating
  symlink/`os.SameFile` logic (fix.go:106-145 vs filewrite.go:154-177).
- [ ] `source.DisplayPath` calls `os.Getwd()` per invocation and is used in
  per-file loops — resolve the working directory once.
- [ ] `config`: replace the `definedOptions` / `HasExplicitOption` ceremony
  (config.go:80, 166-179) with pointer fields (`*int`) on `RuleConfig` so
  "unset" is representable; derive the per-check allowed-options validation
  from the single source created in Phase 1b instead of the parallel
  hardcoded lists (config.go:20-29, 275-282, `cloneRuleConfig`). Consider
  renaming the `[check]` tool table to `[tool]` or nesting per-rule settings
  as `[check.rules.<code>]` — the singular/plural `check`/`checks` pair is a
  one-character trap.
- [ ] `baseline`: keep the feature (it earns its keep for adoption), but
  export one entry point, replace `\x00`-joined string keys with a comparable
  struct (baseline.go:220-222), and remove the one-value `Variant` type.

## Phase 5 — Formatter and CST cleanups

- [ ] **Split `concrete_printer.go` (844 lines, five concerns)** into
  `layout.go`, `render.go`, `writer.go`, `imports.go`, and
  `module_cache.go` (the tests for the module cache already point at that
  missing file name). Filesystem I/O (`modulePathIn` reading `go.mod`) does
  not belong in a printer file.
- [ ] Replace the 13-parallel-boolean-slices layout storage
  (concrete_printer.go:221-241) with a per-token struct — same performance,
  no hand-counted slice bounds.
- [ ] Unify the duplicated declaration-rank tables
  (`concreteDeclarationRank` safety.go:132-146 vs
  `formatterDeclarationRank` declaration_order.go:104-122) — if they drift,
  the safety check validates the wrong invariant. Same for the duplicated
  import-spec walk (safety.go:71-91 vs concrete_printer.go:480-501).
- [ ] **Question: top-level declaration reordering.** The formatter *moves
  code* (const→var→type→func), which destroys intentional grouping, inflates
  first-run diffs, and forces the sorted-comment fingerprint hack in
  `safety.go`. Recommendation: make it a check with an automatic fix (the
  semantic package already has `top-level-declaration-order`!) and remove it
  from the format pass. This deletes `declaration_order.go`'s parser round
  trip and simplifies `safety.go` substantially.
- [ ] Make `internal/fix` use a shared formatter session instead of the free
  `Format` function in a loop (fix.go:305), which allocates a fresh module
  cache per file.
- [ ] Remove the double `IsIgnored`/`normalizeOptions` calls on the
  `FormatTree` path (formatter.go:83/169, 121/158).
- [ ] Investigate the convergence loop (up to 100 re-renders,
  formatter.go:188-207): a formatter needing fixpoint iteration means layout
  decisions depend on prior output. At minimum document why; ideally make
  `shouldBreak` deterministic in one pass. Related: the formatter's own
  output splits short-var-decl LHS across lines
  (`spec,\n  isSpec := ...`) throughout this repo — the style rule that
  produces this should be revisited.
- [ ] `cst`: the reflection fallbacks (~250 lines: `collectChildren`,
  `appendChildrenReverse`, `collectTokenBounds`, three generic variants)
  exist only for test-only foreign node types. Consider deleting them and
  letting gencst's hard-fail guarantee coverage. Keep `cmd/gencst` as is —
  it is the strongest code in the repo (schema hash, dependency-version
  pinning, CI check).
- [ ] Consider exporting a `cst.IsArguments(node)`-style helper so consumers
  stop writing `strings.HasPrefix(cst.Kind(x), "Arguments")` in four places.

## Phase 6 — Right-size the incremental/watch machinery

`semantic/session.go` (632 lines) + `session_probe.go` (427 lines) implement
a SHA-256-everything cache (full `os.Environ()`, `go env -json`, package
graph, every source byte, ancestor `go.mod` walks to filesystem root) — for a
cache capped at **8 entries** whose hit path can still run two full metadata
loads. `check --watch` (check.go:271-351) polls every second and re-reads
every file's bytes per tick.

- [ ] Decide what watch mode is for. If it stays: replace 1s full-rehash
  polling with fsnotify, and simplify the fingerprint to file
  (path, size, mtime) + config + go.mod/go.work hashes. That removes most of
  `session_probe.go` and the ~80%-duplicated graph-hashing between
  `session.go:391-466` and `session_probe.go:149-225`.
- [ ] If watch mode is not a priority, delete the semantic `Session` cache
  entirely and let watch mode re-run analysis — simplicity over a
  correct-but-disproportionate 1,000-line cache. (Keep the cheap concrete
  cache in `checks/session.go`, which is small and proportionate.)

## Phase 7 — Question the check inventory

We don't have to solve every problem. Candidates to demote or delete:

- [ ] **Style checks living in the semantic (type-checked) engine** pay full
  `packages.Load` cost for no reason: `task-comment`, `doc-comment-period`,
  `top-level-declaration-order`, `excessive-blank-identifiers`. Move to the
  syntax pass or drop.
- [ ] **`test-parallelism`** — 100+ lines of escape-analysis heuristics to
  emit an advisory "consider t.Parallel()". High false-positive surface,
  note severity. Recommend deletion.
- [ ] **`external-call-in-loop`** — external calls in loops are usually
  intentional; warning severity invites noise. Recommend deletion or note
  severity.
- [ ] **`optimize-operands-order`** (syntax) — the cost heuristic flags
  intentional short-circuit ordering. Recommend deletion.
- [ ] **`discarded-pure-result`'s purity table** misclassifies `time.Now` /
  `time.Parse` as pure (discarded_pure_result.go:45-47) and carries an
  interprocedural SSA purity checker. Shrink the table to unambiguous cases
  or drop the interprocedural part.
- [ ] The `go-flags` third-party whitelist inside a generic struct-tag check
  (cst_struct_rules.go:66) — remove the special case.
- [ ] `unclosed_resources.go` (912 lines) is a hand-rolled path-sensitive
  dataflow engine for two checks. Either promote the engine to a named,
  documented internal package or simplify the checks to a cheaper
  approximation. A mini abstract interpreter hidden in one rule file is a
  maintenance trap.
- [ ] `deprecated_api_usage.go` (540 lines) embeds a whole-program
  deprecation harvester (re-parsing GOROOT sources) inside a check file;
  the engine calls into it (semantic.go:88-92). If the check stays, the
  harvester is engine infrastructure and should live beside the engine.

## Phase 8 — Tests

- [ ] Split `semantic_test.go` (3,430 lines) and `app_test.go` (1,893 lines)
  per check/command; colocate check tests with the check file so grep isn't
  the test-discovery mechanism.
- [ ] Replace count-based assertions (`len(diagnostics) != 4`) with
  position-anchored expectations (`// want`-style comments or golden files);
  failures become localizable and refinements stop breaking unrelated tests.
- [ ] Remove `os.Chdir`-based test setup in semantic tests
  (`semantic_test.go:3406-3430`) — it forces serial execution; use
  `packages.Config.Dir` (and `Overlay` where possible) instead.
- [ ] Replace change-detector tests with intention-revealing ones: the
  hardcoded rule counts (`requirements_test.go:41`, `syntax_test.go:967`) and
  the SHA-256 catalog digest (`syntax_test.go:1024-1027`) fail without a
  diff. A golden list of check codes gives an actual diff on failure.
- [ ] Add the missing formatter tests noted in review: `Session` reuse across
  modules, direct unit tests for `safety.go`'s fingerprint (currently a bug
  making `equivalentTrees` always pass would go unnoticed).

---

## Sequencing summary

| Phase | Theme | Risk | Payoff |
|---|---|---|---|
| 0 | Deletions, dead code, fake options | Low | ~1,000+ lines gone, smaller surface for everything after |
| 1 | Unified check API | Medium-high | The central goal; one contract, one catalog entry per check |
| 2 | Naming ("check" everywhere, files by content) | Low | Readability; mostly mechanical after Phase 1 |
| 3 | Split `internal/app` | Medium | SRP; one write path; declarative flag/conflict handling |
| 4 | Package boundaries, cross-package duplication | Medium | One implementation per concept |
| 5 | Formatter/CST decomposition | Medium | Printer readable; reorder feature moved to a fix |
| 6 | Watch/session machinery | Medium | Either fsnotify or −1,000 lines |
| 7 | Check inventory pruning | Low | Less noise, less maintenance |
| 8 | Test restructuring | Low | Faster, localizable failures |

Validation after each step: `make check && make test`. After each completed
phase: `make corpus-check`, and if green, `make corpus-update`.

Guiding rules going forward:

1. One noun: **check**. One file per check. Metadata, behavior, options, and
   requirements declared together.
2. Adding a check touches exactly two files: the check file and its test.
3. Every concept has one implementation and one home (generated-file
   detection, severity styling, atomic writes, path display, module lookup).
4. No configuration knob with exactly one legal value.
5. No file named after the work session that produced it.
6. The engine must pass its own linter.
