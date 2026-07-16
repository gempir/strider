---
title: string-format
description: enforce configured string constraints.
---

Purpose: enforce configured string constraints.

## Behavior

Strider implements this rule natively in its shared lossless Go CST traversal. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

no constraints. The rule is part of Strider's extended catalog and runs
when selected with `--only string-format` or when the complete catalog is enabled
with `--all-rules`.
