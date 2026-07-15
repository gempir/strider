---
title: line-length-limit
description: limit source line length.
---

Purpose: limit source line length.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

maximum 80 runes. The rule is part of Strider's extended catalog and runs
when selected with `--only line-length-limit` or when the complete catalog is enabled
with `--all-rules`.
