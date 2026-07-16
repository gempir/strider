---
title: context-as-argument
description: place context.Context first in parameter lists.
---

Purpose: place context.Context first in parameter lists.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

enabled. The rule is part of Strider's extended catalog and runs
when selected with `--only context-as-argument` or when the complete catalog is enabled
with `--all-rules`.
