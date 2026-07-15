---
title: use-waitgroup-go
description: prefer WaitGroup.Go.
---

Purpose: prefer WaitGroup.Go.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

Go 1.25 and newer. The rule is part of Strider's extended catalog and runs
when selected with `--only use-waitgroup-go` or when the complete catalog is enabled
with `--all-rules`.
