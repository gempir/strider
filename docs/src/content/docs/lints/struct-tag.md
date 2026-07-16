---
title: struct-tag
description: Validate struct tag syntax, duplicate keys, and standard options.
---

Purpose: detect malformed struct tags before reflection or encoding packages
silently ignore them.

## Behavior

The rule validates quoted key-value syntax, duplicate keys, whitespace and
quoting in common tag names, and JSON/XML options. Repeated `choice`,
`optional-value`, and `default` keys are accepted in files importing
`github.com/jessevdk/go-flags`, whose tag format intentionally uses them.

## Default

standard tags. The rule is part of Strider's extended catalog and runs
when selected with `--only struct-tag` or when the complete catalog is enabled
with `--all-rules`.
