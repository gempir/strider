# Contributing

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
their Go modules outside the timed section, and then runs these commands with
`GOMAXPROCS=2` and project configuration disabled:

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
