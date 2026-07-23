# Contributing

## Authoring a check

Start a check with `checkscaffold`. Supply complete user-facing metadata up
front so the command can validate the global code namespace and produce its
reference page:

```sh
go run ./scripts/checkscaffold \
  -engine semantic \
  -stage types \
  -code missing-package-context \
  -summary "detect package operations without context" \
  -explanation "Package operations can block; require a caller-owned context while leaving wrappers and generated code unreported." \
  -good 'load(ctx, name)' \
  -bad 'load(context.Background(), name)' \
  -severity warning
```

Use `-engine syntax` for a CST check. Semantic checks select `-stage types`
or `-stage ssa`. Simple options can be declared without another defaults table:

```sh
-options-json '[{"name":"allowed","kind":"strings","default_strings":[],"help":"Names exempt from this check."}]'
```

Supported scaffold option kinds are `int` (a non-negative integer) and
`strings`. The command rejects incomplete metadata, invalid option schemas,
existing file/doc targets, and codes that collide case-insensitively. It then
updates the central generated registration, inventory goldens, reference page,
and catalog statistics. Review those generated diffs, but author behavior in
only the two new files:

```text
internal/checks/<engine>/<code>.go
internal/checks/<engine>/<code>_test.go
```

The generated implementation intentionally does nothing. Replace it and the
metadata-only scaffold test with focused positive and adversarial cases.

For a syntax check, keep traversal ownership in the shared CST engine. The
scaffold registers a `SourceFile` interest; narrow it to the smallest node kind
when implementing the check:

```go
func (pass *Pass) checkPackageBanner(file *cst.SourceFile) {
	if file.TopLevelDeclList == nil {
		pass.Report(file, "package contains no declarations")
	}
}
```

Test syntax behavior through its selected registry so dispatch and reporting
are covered:

```go
filename := writeFixture(t, "package empty\n")
registry, err := NewRegistry([]string{"package-banner"})
diagnostics, err := Run([]string{filename}, registry)
```

For a semantic check, implement only `Meta` and `Run`; registration owns
whether the prepared pass stops at types or includes SSA:

```go
func (missingPackageContextCheck) Run(pass *Pass) {
	pass.Inspect([]ast.Node{(*ast.CallExpr)(nil)}, func(node ast.Node) bool {
		call := node.(*ast.CallExpr)
		if isPackageFunction(pass.TypesInfo, call.Fun, "context", "Background") {
			pass.Report(call, "accept a caller-owned context")
		}
		return true
	})
}
```

Use `assertCheckDiagnostics` for a compact positive fixture, then add negative
forms that define the false-positive boundary. Checks with exact positions or
fixes should call `assertDiagnosticGolden`.

Refresh every diagnostic, engine-code, unified-inventory, and JSON golden with
the one documented target:

```sh
make golden-update
git diff -- internal/checks internal/report
```

`make check-update` additionally regenerates check documentation and catalog
statistics. `checkscaffold` runs it automatically. Finish with the same
validation used in CI:

```sh
make check
make test
```

## Test suites

Run the unit, integration, and package tests with:

```sh
make test
```

The open-source corpus checks 11 pinned Go projects with Strider's formatter
and unified check engine:

```sh
make corpus-check
```

The runner clones projects into the gitignored `.benchmark-cache/`, downloads
their Go modules outside the timed section, and then runs these commands for a
pinned `linux/amd64` target with CGO disabled, `GOMAXPROCS=2`, and project
configuration disabled:

```sh
strider --no-config fmt --check .
strider --no-config check --minimum-severity note --format json .
```

Exit codes 0 and 1 are valid outcomes: formatting differences and diagnostics
are findings. Exit code 2, malformed JSON, checkout failures, and package-load
failures are suite errors. The baseline compares each exit code and a SHA-256
digest of normalized stdout and stderr, plus finding totals and per-rule counts.

When an intentional Strider change alters results, review `target/corpus/`, then
accept the new behavior and refresh the docs reports with:

```sh
make corpus-update
git diff -- benchmarks/baseline.json docs/public/benchmark-report/ docs/src/generated/kubernetes-benchmark.json
```

This also regenerates one detailed report per project under
`docs/public/benchmark-report/projects/`. Each report combines operation timings
with lint and analysis diagnostics and source context resolved from the pinned
checkout. The same run exports Kubernetes format and check timings for the
homepage. Keep the matching Starlight project page under
`docs/src/content/docs/benchmarks/` when changing the corpus manifest.

Do not accept a baseline just to make CI green. Unexpected changes often reveal
ordering bugs, rule regressions, or formatter compatibility issues.

## Performance budgets

Each project has separate format and check budgets in
`benchmarks/projects.json`. The corpus check fails when an operation exceeds its
budget. GitHub Actions writes every measurement and threshold to the job summary
and uploads the JSON and HTML reports for 30 days.

Budgets are intentionally above ordinary GitHub-hosted runner times to detect
meaningful regressions rather than scheduler noise. Change a budget only after
reviewing several CI runs and explain the reason in the commit.
