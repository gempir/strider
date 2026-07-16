---
title: errorf
description: replace errors.New around fmt.Sprintf.
---

Purpose: replace errors.New around fmt.Sprintf.

## Behavior

Strider implements this rule natively in its shared lossless Go CST traversal. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

enabled. The rule is part of Strider's extended catalog and runs
when selected with `--only errorf` or when the complete catalog is enabled
with `--all-rules`.
