---
title: empty-block
description: Detect empty statement blocks that need removal or explanation.
---

**Default severity:** 🟡 `warning`

Purpose: catch empty `if` and `else` branches that are usually unfinished code
or accidental no-ops.

## Behavior

Empty functions, methods, closures, and loop bodies are accepted because they
are commonly intentional stubs, marker methods, callbacks, or drain loops and
have different semantics from an empty conditional branch.

## Enable

This optional check runs when selected with `--only empty-block`, enabled in
`strider.toml`, or included with `--all`.
