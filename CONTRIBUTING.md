# Contributing

## Test suites

Run the unit, integration, and package tests with:

```sh
make test
```

The open-source corpus checks 10 pinned Go projects with all three Strider
engines:

```sh
make corpus-check
```

The runner clones projects into the gitignored `.benchmark-cache/`, downloads
their Go modules outside the timed section, and then runs these commands with
`GOMAXPROCS=2` and project configuration disabled:

```sh
strider fmt --check .
strider lint --all-rules --format json .
strider analyze --format json .
```

Exit codes 0 and 1 are valid outcomes: formatting differences and diagnostics
are findings. Exit code 2, malformed JSON, checkout failures, and package-load
failures are suite errors. The baseline compares each exit code and a SHA-256
digest of normalized stdout and stderr, plus finding totals and per-rule counts.

When an intentional Strider change alters results, review `target/corpus/`, then
accept the new behavior and refresh the docs report with:

```sh
make corpus-update
git diff -- benchmarks/baseline.json docs/public/benchmark-report/index.html
```

Do not accept a baseline just to make CI green. Unexpected changes often reveal
ordering bugs, rule regressions, or formatter compatibility issues.

## Performance budgets

Each project has separate format, lint, and analyze budgets in
`benchmarks/projects.json`. The corpus check fails when an operation exceeds its
budget. GitHub Actions writes every measurement and threshold to the job summary
and uploads the JSON and HTML reports for 30 days.

Budgets are intentionally above ordinary GitHub-hosted runner times to detect
meaningful regressions rather than scheduler noise. Change a budget only after
reviewing several CI runs and explain the reason in the commit.
