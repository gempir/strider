---
title: enforce-switch-style
description: require default clauses to be last.
---

Purpose: require default clauses to be last.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

default optional. The rule is part of Strider's extended catalog and runs
when selected with `--only enforce-switch-style` or when the complete catalog is enabled
with `--all-rules`.
