---
title: unexported-return
description: avoid exported APIs returning private types.
---

Purpose: avoid exported APIs returning private types.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

enabled. The rule is part of Strider's extended catalog and runs
when selected with `--only unexported-return` or when the complete catalog is enabled
with `--all-rules`.
