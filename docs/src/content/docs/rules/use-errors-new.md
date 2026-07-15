---
title: use-errors-new
description: prefer errors.New for static errors.
---

Purpose: prefer errors.New for static errors.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

enabled. The rule is part of Strider's extended catalog and runs
when selected with `--only use-errors-new` or when the complete catalog is enabled
with `--all-rules`.
