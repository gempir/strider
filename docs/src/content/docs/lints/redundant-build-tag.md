---
title: redundant-build-tag
description: remove redundant legacy build tags.
---

Purpose: remove redundant legacy build tags.

The rule reports a legacy `+build` line when a modern `go:build` constraint is
present, and reports repeated legacy constraints whose terms are equivalent
even when written in a different order.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

enabled. The rule is part of Strider's extended catalog and runs
when selected with `--only redundant-build-tag` or when the complete catalog is enabled
with `--all-rules`.
