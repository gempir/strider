---
title: range-val-in-closure
description: avoid capturing range values in closures.
---

Purpose: avoid capturing range values in closures.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

enabled. The rule is part of Strider's extended catalog and runs
when selected with `--only range-val-in-closure` or when the complete catalog is enabled
with `--all-rules`.
