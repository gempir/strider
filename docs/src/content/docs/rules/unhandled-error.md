---
title: unhandled-error
description: detect ignored error-returning calls.
---

Purpose: detect ignored error-returning calls.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

common functions. The rule is part of Strider's extended catalog and runs
when selected with `--only unhandled-error` or when the complete catalog is enabled
with `--all-rules`.
