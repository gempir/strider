<img src="docs/src/assets/strider.png" alt="Strider" width="200">

# Strider

Strider is an experimental Go 1.26 toolchain with a strict formatter, an
AST-only linter, and package-aware static analysis.

# Slopclaimer

This is slop, written heavily with LLMs. I don't have the time next to a full time job to 
build this level of product without LLMs. The good thing though, none of this code ever runs in production.
You run it in CI or locally and get useful output or not. 

## Inspiration

Strider is hugely inspired by
[carthage-software/mago](https://github.com/carthage-software/mago), particularly
its speed, configuration design, and clear separation between formatting,
linting, analysis, and reporting.

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

## Analyze

```sh
strider analyze ./...
strider analyze --format json ./...
strider analyze --only SA1000 ./...
strider analyze --list-rules
strider analyze --explain SA1000
```

`analyze` loads complete packages, type-checks them, and builds SSA before
running deeper correctness and data-flow checks. The first implemented check
is Staticcheck-compatible `SA1000`, which detects invalid constant regular
expressions passed to `regexp.Compile`, `regexp.MustCompile`, `regexp.Match`,
`regexp.MatchReader`, and `regexp.MatchString`.

## Exit codes

- `0`: success with no findings or formatting differences.
- `1`: lint/analyze findings or files that differ in `--check`/`--diff` mode.
- `2`: command, parsing, unsupported-syntax, or I/O error.

Configuration, vet integration, analysis baselines, and staged-file workflows
remain deferred while the command contracts are evolving.
