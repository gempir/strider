---
title: unnecessary-format
description: avoid formatting calls without directives.
---

Purpose: avoid formatting calls without directives.

## Behavior

Strider implements this rule natively in its shared lossless Go CST traversal. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

enabled. The rule is part of Strider's extended catalog and runs
when selected with `--only unnecessary-format` or when the complete catalog is enabled
with `--all-rules`.
