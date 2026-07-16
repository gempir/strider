---
title: max-public-structs
description: limit exported structs per file.
---

Purpose: limit exported structs per file.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

maximum 5. The rule is part of Strider's extended catalog and runs
when selected with `--only max-public-structs` or when the complete catalog is enabled
with `--all-rules`.
