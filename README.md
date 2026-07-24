<img src="docs/src/assets/strider.png" alt="Strider" width="200">

# Strider

Strider is an experimental Go 1.26 formatter and code checker. One
`strider check` run reports formatting drift, style and maintainability issues.
It is intentionally very opinionated and picky out of the box, configure as you like.
The formatter is even more opinionated, kinda like a gofmt but stricter.

# Slopclaimer

This is slop, written heavily with LLMs. I don't have the time next to a full
time job to build this level of product without LLMs. The good thing though,
none of this code ever runs in production. You run it in CI or locally and get
useful output or not.

## Inspiration

Strider is hugely inspired by
[carthage-software/mago](https://github.com/carthage-software/mago), particularly
its speed, configuration design, and reporting.

## Check

```sh
strider check
strider check --format json
strider check --format html ./... > check-report.html
strider check -s error

strider check --help
Usage: strider check [OPTIONS] [FILE|DIR]...
  -b, --baseline VALUE
      path to the check baseline
  -e, --explain VALUE
      explain a check
  -x, --fix
      apply safe automatic fixes
  -u, --fix-unsafe
      apply all automatic fixes, including unsafe fixes
  --no-package-loading
      skip package-aware checks
  -f, --format VALUE
      report format: text, json, or html (default "text")
  -g, --generate-baseline
      replace the baseline with all current findings
  -l, --list-checks
      list checks admitted by the severity floor
  -s, --minimum-severity VALUE
      minimum effective severity: none, note, warning, or error
  -o, --only VALUE
      run only these check codes (repeatable or comma-separated)
  -r, --remove-outdated-baseline-entries
      remove baseline entries that no longer match
  -q, --summary-only
      only print per-check counts and the aggregate issue summary
  -w, --watch
      rerun checks when source changes
```

The first `strider check` run in a Go module can take longer while Go loads
packages and downloads dependencies. Later runs are usually faster because Go
reuses its module and build caches. Use `--no-package-loading` to run only
formatting and syntax checks when package metadata or dependencies are
unavailable.

## Configuration

Strider discovers the nearest `strider.toml` from the current directory upward.
Version 1 uses `[check]` for command-wide policy and `[checks.<code>]` for
individual checks. Every check supports `severity` and path `excludes`; the
formatter exposes only its selected width and filesystem exclusions.

```toml
version = 1
color = "auto"

[formatter]
print-width = 180
excludes = ["internal/generated/**"]

[check]
baseline = "strider-baseline.toml"
minimum-severity = "warning"

[checks.file-length-limit]
severity = "warning"
max-lines = 800

[checks.unclosed-http-response-body]
severity = "error"
excludes = ["internal/legacy/**"]
```


## Developement

### Create a check

Use the scaffold command to start a built-in check with its metadata, registry
entry, test, and documentation page:

```sh
go run ./scripts/checkscaffold \
  -engine semantic \
  -stage types \
  -code missing-package-context \
  -summary "detect package operations without context" \
  -explanation "Package operations should use a caller-owned context; uncertain forms are ignored." \
  -good 'load(ctx, name)' \
  -bad 'load(context.Background(), name)' \
  -severity warning
```

Use `-engine syntax` for CST checks, or choose `-stage ssa` for an SSA-based
semantic check. Replace the generated no-op implementation and metadata-only
test in `internal/checks/<engine>/`, adding focused positive and adversarial
cases. The scaffold refreshes generated docs and golden data; finish with
`make check` and `make test`.

### Open-source corpus

`make corpus-check` runs formatting and the check catalog against pinned popular
Go projects. Each category uses a warm-up plus seven measured samples across
fixed two-core/native scheduling and cold/warm Strider cache states; package
checks also separate cold and warm Go build caches. Reports retain raw samples,
median and p95 latency, allocations, GC work, external peak RSS, the build
identity, and the benchmark environment. The command rejects processing errors,
compares deterministic output with a reviewed baseline, and enforces
per-project timing budgets.
