---
title: package-naming
description: enforce conventional package names.
---

Purpose: enforce conventional package names.

## Behavior

Strider implements this rule natively in its shared lossless Go CST traversal. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

lower-case letters and digits. The rule is part of Strider's extended catalog and runs
when selected with `--only package-naming` or when the complete catalog is enabled
with `--all-rules`.
