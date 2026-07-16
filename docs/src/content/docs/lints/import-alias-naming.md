---
title: import-alias-naming
description: enforce conventional import aliases.
---

Purpose: enforce conventional import aliases.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

lower-case letters and digits. The rule is part of Strider's extended catalog and runs
when selected with `--only import-alias-naming` or when the complete catalog is enabled
with `--all-rules`.
