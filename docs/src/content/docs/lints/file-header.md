---
title: file-header
description: require a configured source header.
---

Purpose: require a configured source header.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

no required header. The rule is part of Strider's extended catalog and runs
when selected with `--only file-header` or when the complete catalog is enabled
with `--all-rules`.
