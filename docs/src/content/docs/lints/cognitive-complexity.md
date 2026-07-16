---
title: cognitive-complexity
description: limit nested control-flow complexity.
---

Purpose: limit nested control-flow complexity.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

maximum 7. The rule is part of Strider's extended catalog and runs
when selected with `--only cognitive-complexity` or when the complete catalog is enabled
with `--all-rules`.
