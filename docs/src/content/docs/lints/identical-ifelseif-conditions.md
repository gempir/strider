---
title: identical-ifelseif-conditions
description: detect repeated if-chain conditions.
---

Purpose: detect repeated if-chain conditions.

Chains containing initializer statements, calls, or channel receives are
accepted because evaluating the same-looking condition again may produce a
different result.

## Behavior

Strider implements this rule natively in its shared Go AST analysis pass. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

enabled. The rule is part of Strider's extended catalog and runs
when selected with `--only identical-ifelseif-conditions` or when the complete catalog is enabled
with `--all-rules`.
