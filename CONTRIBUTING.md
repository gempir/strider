# Contributing

## Test suites

The deterministic suite checks curated formatter output, idempotence, lint
findings, and static-analysis findings using the existing Strider binary:

```sh
make test
```

The Wilds suite exercises pinned open-source projects cloned into the
gitignored `.wilds/` directory:

```sh
make wilds
```

Use `make wilds-all` to run every native rule against each pinned Wilds
project.

Use `make wilds-analyze` to run all implemented package-aware checks, or select
one while developing it:

```sh
make wilds-analyze STRIDER_ANALYZE_ARGS='--only invalid-regexp'
```

`make wilds` is a smoke test. Formatting differences and lint findings are
printed as observations, while crashes and processing errors fail the run.
`make wilds-check` compares exit codes, formatter and linter output
fingerprints, per-rule lint counts, and errors with reviewed baselines. Full
output is printed when a fingerprint changes. `make wilds-accept` explicitly
updates those baselines after review.

Wilds baselines record behavior, not correctness. After deciding whether a
finding is correct, reduce it to a focused case under `testdata/cases/`; those
curated cases are the source of truth used by `make test`.

Every Strider invocation reports elapsed time and enforces a deliberately
generous speed budget. Timing reports are written as TSV files under
`target/timings/`. Override `CURATED_MAX_SECONDS`, `WILDS_FMT_MAX_SECONDS`, or
`WILDS_LINT_MAX_SECONDS` to tune the budgets for a machine. Analyzer runs have
separate `CURATED_ANALYZE_MAX_SECONDS` and `WILDS_ANALYZE_MAX_SECONDS` budgets.
GitHub Actions adds the measurements to the job summary and uploads the reports
as build artifacts.

Add `name,repository,commit` entries to `WILDS_PROJECTS` in the Makefile to
extend the corpus. Override `STRIDER` to test another binary, for example:

```sh
make test STRIDER=/path/to/strider
```
