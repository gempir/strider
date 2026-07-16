---
title: dot-imports
description: discourage dot imports.
---

Purpose: discourage dot imports.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

no allowed packages. The rule is part of Strider's extended catalog and runs
when selected with `--only dot-imports` or when the complete catalog is enabled
with `--all-rules`.
