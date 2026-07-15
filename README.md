![Strider](docs/src/assets/strider.png)

# Strider

Strider is an experimental toolchain that includes a strict formatter and linter for Go 1.26.

# Slopclaimer

This is slop, written heavily with LLMs. I don't have the time next to a full time job to 
build this level of product without LLMs. The good thing though, none of this code ever runs in production.
You run it in CI or locally and get useful output or not. 

## Inspiration

Strider is hugely inspired by
[carthage-software/mago](https://github.com/carthage-software/mago), particularly
its speed, configuration design, and clear separation between formatting,
linting, analysis, and reporting.

## Test suites

The deterministic suite checks curated formatter output, idempotence, known
lint findings, and clean lint input using the existing Strider binary. It does
not invoke Go:

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
`WILDS_LINT_MAX_SECONDS` to tune the budgets for a machine. GitHub Actions adds
the measurements to the job summary and uploads the reports as build artifacts.

Add `name,repository,commit` entries to `WILDS_PROJECTS` in the Makefile to
extend the corpus. Override `STRIDER` to test another binary, for example:

```sh
make test STRIDER=/path/to/strider
```

## Format

```sh
strider fmt --check ./...
strider fmt --diff ./...
strider fmt --write ./...
strider fmt --stdin < file.go
```

With file or directory arguments, formatting writes in place unless `--check`
or `--diff` is used. With no path, Strider recursively scans the current
directory. Use `--stdin` to read source from stdin and write it to stdout. A
file containing `//strider:format-ignore` is passed through unchanged.

The formatter spike supports ordinary, non-generic application code. It
currently rejects generics, `goto`, `fallthrough`, and some comments embedded
inside expressions. Type switches, `select`, channel sends, and labeled
statements are supported.

## Lint

```sh
strider lint ./...
strider lint --format json ./...
strider lint --only no-init,no-package-var ./...
strider lint --list-rules
strider lint --explain cyclomatic-complexity
```

With no path, `lint` also recursively scans the current directory.

The initial rules are `cyclomatic-complexity`, `max-parameters`,
`no-naked-return`, `no-init`, `no-package-var`, `no-defer-in-loop`, and
`no-else-after-return`. Strider also includes 104 additional native rules. Run
the complete 111-rule registry with:

```sh
strider lint --all-rules ./...
```

Use `--only RULE` to select any individual rule without enabling the rest.

Suppress a rule on the next declaration or statement with:

```go
//strider:ignore no-package-var
var ErrUnavailable = errors.New("unavailable")
```

Use `//strider:ignore-file code[,code]` before the package clause for a
file-level suppression.

## Exit codes

- `0`: success with no findings or formatting differences.
- `1`: lint findings or files that differ in `--check`/`--diff` mode.
- `2`: command, parsing, unsupported-syntax, or I/O error.

Configuration, type-aware analysis, vet integration, baselines, and staged-file
workflows are intentionally deferred until the formatter and linter contracts
have proved themselves.
