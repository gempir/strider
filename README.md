<img src="docs/src/assets/strider.png" alt="Strider" width="200">

# Strider

Strider is an experimental Go 1.26 toolchain with a lossless CST formatter and
linter, plus package-aware static analysis built on Go's AST, type information,
and SSA.

# Slopclaimer

This is slop, written heavily with LLMs. I don't have the time next to a full time job to 
build this level of product without LLMs. The good thing though, none of this code ever runs in production.
You run it in CI or locally and get useful output or not. 

## Inspiration

Strider is hugely inspired by
[carthage-software/mago](https://github.com/carthage-software/mago), particularly
its speed, configuration design, and clear separation between formatting,
linting, analysis, and reporting.

## Configuration

Strider discovers the nearest `strider.toml` from the current directory upward.
Every lint rule and analyzer supports `enabled`, `severity`, and path
`excludes`; the formatter exposes print width, visual indentation width, line
endings, and filesystem exclusions.

```toml
version = 1
color = "auto"

[formatter]
print-width = 100
max-empty-lines = 1

[linter.rules.line-length-limit]
enabled = true
severity = "warning"

[analyzer.rules.possible-nil-dereference]
severity = "error"
excludes = ["internal/legacy/**"]
```

Use `strider --config PATH COMMAND` to select a file explicitly or
`strider --no-config COMMAND` to run with built-in defaults. Rich terminal
output uses color automatically; set `color = "always"` or `"never"`, or
override it with `strider --color always|never COMMAND`. `NO_COLOR` and
`FORCE_COLOR` are also honored. The schema is strict: unknown keys and rule
codes are errors.

## Shell completion

Strider generates completion scripts for Bash, Zsh, Fish, and PowerShell:

```sh
strider completion bash
strider completion zsh
strider completion fish
strider completion powershell
```

Each shell command's help includes installation instructions. For example,
`strider completion fish > ~/.config/fish/completions/strider.fish` enables
completion for future Fish sessions. Completions include commands, flags,
fixed values such as `--format json`, paths, and lint/analyzer rule codes.

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

The formatter spike supports ordinary application code, including generics,
type switches, `select`, channel sends, and labeled statements. It currently
rejects `goto`, `fallthrough`, and some comments embedded inside expressions.

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
`no-else-after-return`. Strider also includes 109 additional native rules. Run
the complete 116-rule registry with:

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

## Analyze

```sh
strider analyze ./...
strider analyze --format json ./...
strider analyze --only invalid-regexp ./...
strider analyze --list-rules
strider analyze --explain invalid-regexp
```

`analyze` loads complete packages, type-checks them, and builds SSA before
running deeper correctness and data-flow checks. Analyzer names are descriptive
kebab-case codes such as `invalid-regexp`, `nil-context`, and
`swapped-seek-arguments`.

## Baselines

Lint and analysis baselines record existing findings without hiding new ones:

```sh
strider lint --generate-baseline --baseline lint-baseline.toml ./...
strider analyze --generate-baseline --baseline analysis-baseline.toml ./...
```

Configure the paths for ordinary runs:

```toml
[linter]
baseline = "lint-baseline.toml"
baseline-variant = "loose"

[analyzer]
baseline = "analysis-baseline.toml"
baseline-variant = "loose"
```

Loose baselines match file, code, message, and count while surviving line
movement. Strict baselines match exact line ranges. Use `--ignore-baseline` to
see the full backlog and `--remove-outdated-baseline-entries` to prune fixed
issues without absorbing new findings.

## Exit codes

- `0`: success with no findings or formatting differences.
- `1`: lint/analyze findings or files that differ in `--check`/`--diff` mode.
- `2`: command, parsing, unsupported-syntax, or I/O error.

Vet integration and staged-file workflows remain deferred while those command
contracts are evolving.
