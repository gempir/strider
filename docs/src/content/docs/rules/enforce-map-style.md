---
title: enforce-map-style
description: enforce consistent empty-map construction.
---

Purpose: enforce consistent empty-map construction.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

any style. The rule is part of Strider's extended catalog and runs
when selected with `--only enforce-map-style` or when the complete catalog is enabled
with `--all-rules`.
