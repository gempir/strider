---
title: unsecure-url-scheme
description: detect insecure URL schemes.
---

Purpose: detect insecure URL schemes.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

HTTP, WS, and FTP. The rule is part of Strider's extended catalog and runs
when selected with `--only unsecure-url-scheme` or when the complete catalog is enabled
with `--all-rules`.
