---
title: Lint rules
description: Complete reference for Strider's native Go lint rules.
sidebar:
  order: 0
---

Every profile rule is enabled by default at `warning` severity. Warnings cause
`strider lint` to exit with code `1`. Severity and thresholds are fixed in the
current draft.

| Rule | Purpose | Fixed option |
| --- | --- | --- |
| [`cyclomatic-complexity`](./cyclomatic-complexity/) | Limit independent control-flow paths. | Maximum `10` |
| [`max-parameters`](./max-parameters/) | Limit function parameter count. | Maximum `5` |
| [`no-naked-return`](./no-naked-return/) | Require explicit returned values. | None |
| [`no-init`](./no-init/) | Avoid implicit package initialization. | None |
| [`no-package-var`](./no-package-var/) | Avoid mutable package state. | Blank identifier exempt |
| [`no-defer-in-loop`](./no-defer-in-loop/) | Avoid accumulating deferred calls. | None |
| [`no-else-after-return`](./no-else-after-return/) | Reduce nesting after a terminal return. | None |

Use `strider lint --list-rules` to inspect the executable's registry or
`strider lint --explain CODE` for its short built-in explanation.

## Extended rule catalog

Strider includes 104 additional native rules alongside its seven-rule default
profile. Each rule has a page in this section describing its behavior and
fixed Strider default.

```sh
strider lint --all-rules ./...
```

The extended rules share one native AST traversal per source file.
