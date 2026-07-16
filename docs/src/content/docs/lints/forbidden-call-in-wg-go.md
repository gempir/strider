---
title: forbidden-call-in-wg-go
description: reject panic and Done inside WaitGroup.Go.
---

Purpose: reject panic and Done inside WaitGroup.Go.

## Behavior

Strider implements this rule natively in its shared lossless Go CST traversal. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

enabled. The rule is part of Strider's extended catalog and runs
when selected with `--only forbidden-call-in-wg-go` or when the complete catalog is enabled
with `--all-rules`.
