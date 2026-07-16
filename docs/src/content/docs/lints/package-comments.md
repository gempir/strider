---
title: package-comments
description: require package documentation.
---

Purpose: require package documentation.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

enabled. The rule is part of Strider's extended catalog and runs
when selected with `--only package-comments` or when the complete catalog is enabled
with `--all-rules`.
