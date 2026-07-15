---
title: comment-spacings
description: require a space after line-comment markers.
---

Purpose: require a space after line-comment markers.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

directives exempt. The rule is part of Strider's extended catalog and runs
when selected with `--only comment-spacings` or when the complete catalog is enabled
with `--all-rules`.
