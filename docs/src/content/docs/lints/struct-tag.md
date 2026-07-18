---
title: struct-tag
description: Validate struct tag syntax, duplicate keys, and standard options.
sidebar:
  badge:
    text: error
    class: severity-indicator severity-error
---

Purpose: detect malformed struct tags before reflection or encoding packages
silently ignore them.

**Default severity:** <span class="severity-indicator severity-error" aria-hidden="true"></span> `error`

## Behavior

The rule validates quoted key-value syntax, duplicate keys, whitespace and
quoting in common tag names, and JSON/XML options. Repeated `choice`,
`optional-value`, and `default` keys are accepted in files importing
`github.com/jessevdk/go-flags`, whose tag format intentionally uses them.

## Enable

This optional check runs when selected with `--only struct-tag`, enabled in
`strider.toml`, or included with `--all`.
