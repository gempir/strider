---
title: Style and maintainability checks
description: Reference for Strider's source-level style and maintainability checks.
sidebar:
  order: 0
---

This group contains 116 checks covering style, naming, readability, complexity,
and syntactic maintainability. Seven are enabled in the default `strider check`
profile; architectural and performance risks are warnings, while purely
advisory style findings are notes:

| Check | Purpose | Behavioral default |
| --- | --- | --- |
| [`cyclomatic-complexity`](./cyclomatic-complexity/) | Limit independent control-flow paths. | Maximum `10` |
| [`max-parameters`](./max-parameters/) | Limit function parameter count. | Maximum `5` |
| [`no-naked-return`](./no-naked-return/) (`note`) | Require explicit returned values. | None |
| [`no-init`](./no-init/) (`note`) | Avoid implicit package initialization. | None |
| [`no-package-var`](./no-package-var/) | Avoid mutable package state. | Blank identifier exempt |
| [`no-defer-in-loop`](./no-defer-in-loop/) | Avoid accumulating deferred calls. | None |
| [`no-else-after-return`](./no-else-after-return/) (`note`) | Reduce nesting after a terminal return. | None |

Use `strider check --list-checks` to inspect the enabled registry or
`strider check --explain CODE` for a built-in explanation.

## Optional catalog

The remaining 109 checks are disabled until selected on the CLI or enabled in
configuration. Run every check in this group together with the rest of
Strider's catalog using:

```sh
strider check --all ./...
```

Select one optional check without enabling the others:

```sh
strider check --only add-constant ./...
```

Every page in this section describes one stable check code and its behavioral
contract. Configure codes under `[checks.rules]`; see
[Configuration](/configuration/#checks).
