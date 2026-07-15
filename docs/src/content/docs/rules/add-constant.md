---
title: add-constant
description: suggest named constants for repeated literals.
---

Purpose: suggest named constants for repeated literals.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

strings after 2 repetitions. The rule is part of Strider's extended catalog and runs
when selected with `--only add-constant` or when the complete catalog is enabled
with `--all-rules`.
