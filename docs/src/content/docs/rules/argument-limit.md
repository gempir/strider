---
title: argument-limit
description: limit function parameter count.
---

Purpose: limit function parameter count.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

maximum 8. The rule is part of Strider's extended catalog and runs
when selected with `--only argument-limit` or when the complete catalog is enabled
with `--all-rules`.
