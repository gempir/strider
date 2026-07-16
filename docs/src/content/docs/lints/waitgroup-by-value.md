---
title: waitgroup-by-value
description: pass sync.WaitGroup by pointer.
---

Purpose: pass sync.WaitGroup by pointer.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

enabled. The rule is part of Strider's extended catalog and runs
when selected with `--only waitgroup-by-value` or when the complete catalog is enabled
with `--all-rules`.
