---
title: function-length
description: limit function statements and lines.
---

Purpose: limit function statements and lines.

## Behavior

Strider implements this rule natively in its shared lossless Go CST traversal. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

50 statements; 75 lines. The rule is part of Strider's extended catalog and runs
when selected with `--only function-length` or when the complete catalog is enabled
with `--all-rules`.
