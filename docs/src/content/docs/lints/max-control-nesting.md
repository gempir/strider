---
title: max-control-nesting
description: limit nested control structures.
---

Purpose: limit nested control structures.

## Behavior

Strider implements this rule natively in its shared lossless Go CST traversal. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

maximum depth 5. The rule is part of Strider's extended catalog and runs
when selected with `--only max-control-nesting` or when the complete catalog is enabled
with `--all-rules`.
