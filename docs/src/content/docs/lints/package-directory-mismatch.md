---
title: package-directory-mismatch
description: match package and directory names.
---

Purpose: match package and directory names.

## Behavior

Strider implements this rule natively in its shared lossless Go CST traversal. It
runs entirely inside Strider. Findings use the rule code
Purpose: and warning severity.

## Default

testdata ignored. The rule is part of Strider's extended catalog and runs
when selected with `--only package-directory-mismatch` or when the complete catalog is enabled
with `--all-rules`.
