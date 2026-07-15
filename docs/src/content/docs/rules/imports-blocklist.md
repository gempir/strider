---
title: imports-blocklist
description: reject configured imports.
---

Purpose: reject configured imports.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

empty blocklist. The rule is part of Strider's extended catalog and runs
when selected with `--only imports-blocklist` or when the complete catalog is enabled
with `--all-rules`.
