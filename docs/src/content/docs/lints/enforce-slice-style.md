---
title: enforce-slice-style
description: enforce consistent empty-slice construction.
---

Purpose: enforce consistent empty-slice construction.

## Behavior

Strider implements this rule natively in its shared lossless Go CST traversal. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

any style. The rule is part of Strider's extended catalog and runs
when selected with `--only enforce-slice-style` or when the complete catalog is enabled
with `--all-rules`.
