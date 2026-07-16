---
title: Baselines
description: Adopt Strider without hiding new lint or analysis findings.
---

A baseline records findings that already exist and suppresses only matching
findings on later runs. New findings remain visible and still exit with status
`1`. This makes a baseline useful when introducing Strider to an established
repository, enabling more rules in stages, or tightening CI without fixing the
entire backlog in one change.

Baselines are not blanket ignores. Strider always runs the selected rules,
then consumes matching baseline entries from the current diagnostic set.

## One file per tool

Lint and analysis findings should use different files:

```text
lint-baseline.toml
analysis-baseline.toml
```

The formatter does not use baselines. Its check mode reports byte differences,
not diagnostics with stable rule identities.

## Generate a baseline

Capture the currently selected lint findings:

```sh
strider lint --generate-baseline --baseline lint-baseline.toml ./...
```

Capture analyzer findings separately:

```sh
strider analyze --generate-baseline --baseline analysis-baseline.toml ./...
```

Generation exits `0` and writes no diagnostic report because every current
finding was intentionally captured. The file is written atomically. Existing
content is replaced, so review the diff before committing it.

Generation sees the same effective configuration as an ordinary run:

- Disabled rules are not captured.
- `--only` and `--all-rules` determine the selected rules.
- Tool and per-rule exclusions are applied first.
- Lint source suppressions are applied first.
- Configured severity changes do not affect matching identity.

Generate with the policy CI will actually use. A baseline created with
`--only no-init` cannot suppress unrelated rules later.

## Configure automatic use

Add the paths to `strider.toml` after generating the files:

```toml
[linter]
baseline = "lint-baseline.toml"

[analyzer]
baseline = "analysis-baseline.toml"
```

Then ordinary commands apply them automatically:

```sh
strider lint ./...
strider analyze ./...
```

CLI paths override configured paths:

```sh
strider lint --baseline experiments/lint-baseline.toml ./...
```

Configured relative paths resolve from the directory containing
`strider.toml`. CLI paths resolve from the current working directory. A
configured or explicit baseline must exist and pass strict validation.

## How matching works

Strider supports two variants. The variant is recorded in each file, so
reading does not depend on the current configuration. `baseline-variant`
controls only the next generation.

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

Line numbers are deliberately absent. Adding an import or reformatting code
above an old finding does not invalidate the entry. Counts still protect new
instances: if the baseline contains two matches and a later run finds three,
two are suppressed and the third is reported.

Loose matching is sensitive to diagnostic message changes. A Strider upgrade
that rewords a message may make an old entry stale and report the newly worded
finding. Review and prune or regenerate deliberately.

### Strict: exact line ranges

Strict records the file, rule code, and exact start/end lines:

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
[linter]
baseline-variant = "loose"

[analyzer]
baseline-variant = "strict"
```

Or override it for one generation:

```sh
strider lint --generate-baseline --baseline lint-baseline.toml \
  --baseline-variant strict ./...
```

## See suppressed findings

Temporarily bypass either a configured or explicit baseline:

```sh
strider lint --ignore-baseline ./...
strider analyze --ignore-baseline ./...
```

This is useful for measuring the remaining backlog or choosing the next cleanup
area. It does not modify the baseline.

## Stale entries

When an old finding is fixed, its baseline entry becomes outdated. Strider
still exits based only on visible current findings, but writes a warning to
standard error with the number of stale issues.

Prune only entries that no longer match:

```sh
strider lint --remove-outdated-baseline-entries \
  --baseline lint-baseline.toml ./...
```

The same option works with `analyze`. Pruning is safer than regeneration during
incremental cleanup:

- Matching old entries remain.
- Fixed entries are removed.
- New findings are reported and are **not** added to the baseline.

That last property prevents an accidental baseline refresh from absorbing a
regression.

## Back up before updates

Use `--backup-baseline` with generation or pruning to copy the existing file to
`<path>.bkp` before the atomic replacement:

```sh
strider lint --remove-outdated-baseline-entries --backup-baseline \
  --baseline lint-baseline.toml ./...
```

`--backup-baseline` is rejected without an update option. Generation and
pruning are mutually exclusive, and neither can be combined with
`--ignore-baseline`.

## Recommended adoption workflow

1. Commit `strider.toml` with the intended rule selection and severities.
2. Run lint and analysis without baselines to inspect the size and composition
   of the backlog.
3. Generate separate loose baselines for the exact rule sets CI will run.
4. Review and commit both files.
5. Configure their paths in `strider.toml` and require ordinary lint/analyze
   commands in CI.
6. Fix old findings in focused changes and prune stale entries in the same
   commit.
7. Use `--ignore-baseline` periodically to make the remaining debt visible.

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
