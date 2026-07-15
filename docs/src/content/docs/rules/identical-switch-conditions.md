---
title: identical-switch-conditions
description: detect repeated switch conditions.
---

Purpose: detect repeated switch conditions.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

enabled. The rule is part of Strider's extended catalog and runs
when selected with `--only identical-switch-conditions` or when the complete catalog is enabled
with `--all-rules`.
