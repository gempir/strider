---
title: atomic
description: detect non-atomic operations on atomic values.
---

Purpose: detect non-atomic operations on atomic values.

## Behavior

Strider implements this rule natively in its shared lossless Go CST traversal. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

standard sync/atomic patterns. The rule is part of Strider's extended catalog and runs
when selected with `--only atomic` or when the complete catalog is enabled
with `--all-rules`.
