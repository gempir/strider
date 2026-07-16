---
title: bool-literal-in-expr
description: remove boolean literals from logical comparisons.
---

Purpose: remove boolean literals from logical comparisons.

## Behavior

Strider implements this rule natively in its shared lossless Go CST traversal. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

enabled. The rule is part of Strider's extended catalog and runs
when selected with `--only bool-literal-in-expr` or when the complete catalog is enabled
with `--all-rules`.
