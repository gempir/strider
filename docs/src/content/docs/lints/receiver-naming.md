---
title: receiver-naming
description: enforce consistent receiver names.
---

Purpose: enforce consistent receiver names.

## Behavior

Strider implements this rule natively in its shared lossless Go CST traversal. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

no maximum length. The rule is part of Strider's extended catalog and runs
when selected with `--only receiver-naming` or when the complete catalog is enabled
with `--all-rules`.
