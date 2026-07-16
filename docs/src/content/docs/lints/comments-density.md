---
title: comments-density
description: require a minimum comment density.
---

Purpose: require a minimum comment density.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

minimum 0 percent. The rule is part of Strider's extended catalog and runs
when selected with `--only comments-density` or when the complete catalog is enabled
with `--all-rules`.
