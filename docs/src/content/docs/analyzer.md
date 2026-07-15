---
title: Analyzer
description: Run Strider's package-aware correctness and data-flow checks.
---

`strider analyze` complements the fast AST-only linter. It loads complete Go
packages, resolves types, and constructs SSA so checks can follow constants,
calls, and control flow.

```sh
strider analyze ./...
strider analyze --only SA1000 ./...
strider analyze --format json ./...
```

Use `--list-rules` to see the implemented registry and `--explain CODE` for
the built-in summary and examples. Rule codes follow Staticcheck's canonical
uppercase names but `--only` and `--explain` accept any case.

## Implemented checks

### `SA1000` — invalid regular expression

Reports invalid compile-time constant patterns passed to `regexp.Compile`,
`regexp.MustCompile`, `regexp.Match`, `regexp.MatchReader`, or
`regexp.MatchString`. Constants propagated through local variables are checked;
dynamic patterns are left to runtime.
