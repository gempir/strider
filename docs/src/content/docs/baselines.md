---
title: Baselines
description: Adopt Strider checks without hiding new findings.
---

A baseline records findings that already exist and suppresses only matching
findings on later runs. New findings remain visible and still exit with status
`1`. This lets an established repository adopt Strider, enable more checks in
stages, or tighten CI without fixing the entire backlog in one change.

Baselines are not blanket ignores. Strider always runs the selected checks and
then consumes matching entries from the current diagnostic set.

## One unified baseline

`strider check` uses one file for its diagnostic catalog:

```text
strider-baseline.toml
```

Formatting is intentionally excluded. Code `format` reports byte differences
and should be resolved with `strider fmt`, `[formatter].excludes`, or
`//strider:format-ignore` rather than recorded as debt.

## Generate a baseline

Capture the currently selected non-format findings:

```sh
strider check --generate-baseline --baseline strider-baseline.toml ./...
```

Generation exits `0` and writes no diagnostic report. The file is written
atomically. Existing content is replaced, so review the diff before committing
it. Formatting findings are omitted even if `format` is selected.

Generation sees the same effective configuration as an ordinary run:

- Disabled checks are not captured.
- `--only` and `--all` determine the selected checks.
- `minimum-severity` removes lower-severity checks before generation.
- Tool-wide and per-check exclusions are applied first.
- Supported source suppressions are applied first.
- Configured severity changes do not affect matching identity.

Generate with the policy CI will actually use. A baseline created with
`--only no-init` cannot suppress unrelated checks later.

When applying or pruning an existing baseline, entries for checks omitted by
the current minimum-severity threshold are preserved and are not called stale.
Strider has not run those checks, so their findings cannot be considered fixed.

## Configure automatic use

Add the path to `strider.toml` after generating it:

```toml
version = 1

[checks]
baseline = "strider-baseline.toml"
```

Ordinary runs then apply it automatically:

```sh
strider check ./...
```

A CLI path overrides the configured path:

```sh
strider check --baseline experiments/strider-baseline.toml ./...
```

Configured relative paths resolve from the directory containing
`strider.toml`. CLI paths resolve from the current working directory. A
configured or explicit baseline must exist and pass strict validation.

## How matching works

Strider supports two variants. The variant is recorded in each file, so reading
does not depend on current configuration. `baseline-variant` controls only the
next generation.

### Loose: resilient and count-based

Loose is the default. It groups findings by exact `(file, code, message)` and
stores how many instances currently exist.

```toml
version = 1
variant = "loose"

[[issues]]
file = "internal/server/server.go"
code = "no-package-var"
message = "replace package variable state with explicit dependencies"
count = 2
```

The baseline file format has its own version, independent from configuration
version 1.

Line numbers are deliberately absent. Adding an import or reformatting code
above an old finding does not invalidate the entry. Counts still protect new
instances: if the baseline contains two matches and a later run finds three,
two are suppressed and the third is reported.

Loose matching is sensitive to diagnostic message changes. A Strider upgrade
that rewords a message may make an old entry stale and report the newly worded
finding. Review and prune or regenerate deliberately.

### Strict: exact line ranges

Strict records the file, check code, and exact start and end lines:

```toml
version = 1
variant = "strict"

[[issues]]
file = "internal/server/server.go"
code = "possible-nil-dereference"
start-line = 42
end-line = 42
```

The message may change without invalidating a strict entry, but inserting lines
above it will. Strict is useful when exact location identity matters and the
team accepts more frequent maintenance.

| Variant | Best fit | Main trade-off |
| --- | --- | --- |
| `loose` | Most repositories and CI adoption | Stable across line movement; exact messages and counts matter. |
| `strict` | Small or carefully tracked backlogs | Exact locations; normal edits can make entries stale. |

Choose the generated variant in configuration:

```toml
[checks]
baseline-variant = "strict"
```

Or override it for one generation:

```sh
strider check --generate-baseline --baseline strider-baseline.toml \
  --baseline-variant strict ./...
```

## See suppressed findings

Temporarily bypass a configured or explicit baseline:

```sh
strider check --ignore-baseline ./...
```

This is useful for measuring the remaining backlog or choosing the next cleanup
area. It does not modify the baseline.

## Stale entries

When an old finding is fixed, its baseline entry becomes outdated. Strider
still exits based only on visible current findings but writes a warning to
standard error with the number of stale issues.

Prune only entries that no longer match:

```sh
strider check --remove-outdated-baseline-entries \
  --baseline strider-baseline.toml ./...
```

Pruning is safer than regeneration during incremental cleanup:

- Matching old entries remain.
- Fixed entries are removed.
- New findings are reported and are not added to the baseline.

That last property prevents an accidental baseline refresh from absorbing a
regression.

## Back up before updates

Use `--backup-baseline` with generation or pruning to copy an existing file to
`<path>.bkp` before atomic replacement:

```sh
strider check --remove-outdated-baseline-entries --backup-baseline \
  --baseline strider-baseline.toml ./...
```

`--backup-baseline` is rejected without an update option. Generation and
pruning are mutually exclusive, and neither can be combined with
`--ignore-baseline`.

## Recommended adoption workflow

1. Commit a version-1 `strider.toml` with the intended check selection and
   severities.
2. Run `strider check` without a baseline to inspect the backlog.
3. Generate a loose baseline for the exact profile CI will run.
4. Review and commit the file.
5. Configure its path under `[checks]` and require `strider check` in CI.
6. Fix old findings in focused changes and prune stale entries in the same
   commit.
7. Use `--ignore-baseline` periodically to make remaining debt visible.

Avoid regenerating reflexively after a failure. First determine whether the
finding is an existing issue whose identity legitimately changed or a new
regression that should be fixed.

## File contract

Baseline files are deterministic TOML with `version = 1` and an explicit
`variant`. Issue paths are slash-separated and stored relative to the baseline
file's directory, making committed baselines portable across machines and
checkout locations.

Unknown keys, unsupported versions or variants, missing identity fields,
non-positive loose counts, and invalid strict line ranges are errors. Baseline
parse or I/O errors exit with status `2`; Strider never silently ignores a
broken configured baseline.
