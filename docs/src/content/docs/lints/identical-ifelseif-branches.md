---
title: identical-ifelseif-branches
description: detect repeated if-chain branches.
---

Purpose: detect repeated if-chain branches.

## Behavior

Strider implements this rule natively in its shared lossless Go CST traversal. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

enabled. The rule is part of Strider's extended catalog and runs
when selected with `--only identical-ifelseif-branches` or when the complete catalog is enabled
with `--all-rules`.
