---
title: file-length-limit
description: limit source-file length.
---

Purpose: limit source-file length.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

disabled at 0 lines. The rule is part of Strider's extended catalog and runs
when selected with `--only file-length-limit` or when the complete catalog is enabled
with `--all-rules`.
