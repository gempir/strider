---
title: empty-block
description: Detect empty statement blocks that need removal or explanation.
---

Purpose: catch empty `if` and `else` branches that are usually unfinished code
or accidental no-ops.

## Behavior

Empty functions, methods, closures, and loop bodies are accepted because they
are commonly intentional stubs, marker methods, callbacks, or drain loops and
have different semantics from an empty conditional branch.

## Default

enabled. The rule is part of Strider's extended catalog and runs
when selected with `--only empty-block` or when the complete catalog is enabled
with `--all-rules`.
