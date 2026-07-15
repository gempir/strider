---
title: unused-receiver
description: detect unused method receivers.
---

Purpose: detect unused method receivers.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

enabled. The rule is part of Strider's extended catalog and runs
when selected with `--only unused-receiver` or when the complete catalog is enabled
with `--all-rules`.
