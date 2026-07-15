---
title: filename-format
description: enforce Go source filename format.
---

Purpose: enforce Go source filename format.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

conventional characters. The rule is part of Strider's extended catalog and runs
when selected with `--only filename-format` or when the complete catalog is enabled
with `--all-rules`.
