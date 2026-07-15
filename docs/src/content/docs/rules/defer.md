---
title: defer
description: detect common defer mistakes.
---

Purpose: detect common defer mistakes.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

all checks enabled. The rule is part of Strider's extended catalog and runs
when selected with `--only defer` or when the complete catalog is enabled
with `--all-rules`.
