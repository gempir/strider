---
title: var-naming
description: enforce idiomatic identifier naming.
---

Purpose: enforce idiomatic identifier naming.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

common initialisms. The rule is part of Strider's extended catalog and runs
when selected with `--only var-naming` or when the complete catalog is enabled
with `--all-rules`.
