---
title: call-to-gc
description: discourage explicit garbage collection.
---

Purpose: discourage explicit garbage collection.

## Behavior

Strider implements this rule natively in its shared lossless Go CST traversal. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

runtime.GC. The rule is part of Strider's extended catalog and runs
when selected with `--only call-to-gc` or when the complete catalog is enabled
with `--all-rules`.
