# Strider

Strider is an experimental strict formatter and syntax linter for Go 1.26. It
is built as one dependency-free binary for the initial engine spike.

## Inspiration

Strider is hugely inspired by
[carthage-software/mago](https://github.com/carthage-software/mago), particularly
its speed, configuration design, and clear separation between formatting,
linting, analysis, and reporting.

The formatter deliberately owns its style instead of reproducing `gofmt`
byte-for-byte. Before returning formatted source, it reparses the output,
checks that the syntax tree and comments are unchanged, and verifies that a
second formatting pass is identical. The spike refuses syntax it does not yet
support instead of partially formatting it.

## Build

```sh
CGO_ENABLED=0 go build -trimpath -o strider ./cmd/strider
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
currently rejects generics, type switches, `select`, channel sends, labels,
`goto`, `fallthrough`, and comments embedded inside expressions.

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
`no-else-after-return`.

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
