---
title: unnecessary-stmt
description: detect redundant control flow.
---

Purpose: detect redundant control flow.

## Behavior

Strider implements this rule natively in its shared lossless Go CST traversal. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

enabled. The rule is part of Strider's extended catalog and runs
when selected with `--only unnecessary-stmt` or when the complete catalog is enabled
with `--all-rules`.
