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

- Checks below the effective minimum severity are not captured.
- `--only` can narrow the selected checks.
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

Strider baselines are always strict. Each entry records the file, check code,
and exact start and end lines:

```toml
version = 1
variant = "strict"

[[issues]]
file = "internal/server/server.go"
code = "possible-nil-dereference"
start-line = 42
end-line = 42
```

The baseline file format has its own version, independent from configuration
version 1. A diagnostic message may change without invalidating an entry, but
inserting lines above it will make the exact location stale.

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

## Recommended adoption workflow

1. Commit a version-1 `strider.toml` with the intended check selection and
   severities.
2. Run `strider check` without a baseline to inspect the backlog.
3. Generate a baseline for the exact profile CI will run.
4. Review and commit the file.
5. Configure its path under `[checks]` and require `strider check` in CI.
6. Fix old findings in focused changes and prune stale entries in the same
   commit.

Avoid regenerating reflexively after a failure. First determine whether the
finding is an existing issue whose identity legitimately changed or a new
regression that should be fixed.

## File contract

Baseline files are deterministic TOML with `version = 1` and an explicit
`variant`. Issue paths are slash-separated and stored relative to the baseline
file's directory, making committed baselines portable across machines and
checkout locations.

Unknown keys, unsupported versions or variants, missing identity fields, and
invalid line ranges are errors. Baseline parse or I/O errors exit with status
`2`; Strider never silently ignores a broken configured baseline.
